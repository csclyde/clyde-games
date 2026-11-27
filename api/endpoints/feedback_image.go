package endpoints

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"sync"
)

const maxFeedbackImage = 5 << 20
const maxFeedbackJSON = 64 << 10
const maxFeedbackMultipart = maxFeedbackImage + maxFeedbackJSON + (16 << 10)

var errFeedbackLimit = errors.New("Feedback upload exceeds the size limit.")
var errFeedbackImage = errors.New("Use a valid PNG or JPEG screenshot, at most 4096 pixels per side and 12 megapixels.")
var errFeedbackBusy = errors.New("Too many screenshot uploads. Try again shortly.")
var imageWork = make(chan struct{}, 3)
var multipartWork = make(chan struct{}, 4)
var imageRate = struct {
	sync.Mutex
	last   time.Time
	tokens int
}{tokens: 12}

func allowFeedbackImage(now time.Time) bool {
	imageRate.Lock()
	defer imageRate.Unlock()
	const interval = 2 * time.Second
	if imageRate.last.IsZero() {
		imageRate.last = now
	}
	if gained := int(now.Sub(imageRate.last) / interval); gained > 0 {
		imageRate.tokens += gained
		if imageRate.tokens > 12 {
			imageRate.tokens = 12
		}
		imageRate.last = imageRate.last.Add(time.Duration(gained) * interval)
	}
	if imageRate.tokens == 0 {
		return false
	}
	imageRate.tokens--
	return true
}

var discordImageWork = make(chan struct{}, 3)

func readFeedbackBytes(r io.Reader, max int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, max+1))
	if len(b) > int(max) {
		return nil, errFeedbackLimit
	}
	return b, err
}

func normalizeFeedbackImage(data []byte) ([]byte, error) {
	if len(data) > maxFeedbackImage {
		return nil, errFeedbackLimit
	}
	select {
	case imageWork <- struct{}{}:
		defer func() { <-imageWork }()
	default:
		return nil, errFeedbackBusy
	}
	conf, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || conf.Width < 1 || conf.Height < 1 ||
		conf.Width > 4096 || conf.Height > 4096 || conf.Width*conf.Height > 12_000_000 {
		return nil, errFeedbackImage
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Dx() != conf.Width || img.Bounds().Dy() != conf.Height {
		return nil, errFeedbackImage
	}
	var encoded bytes.Buffer
	// Re-encode without original metadata, trailing bytes or executable formats.
	if err = jpeg.Encode(&encoded, img, &jpeg.Options{Quality: 88}); err != nil {
		return nil, errFeedbackImage
	}
	if encoded.Len() > maxFeedbackImage {
		return nil, errFeedbackLimit
	}
	return encoded.Bytes(), nil
}

func parseFeedbackUpload(c *gin.Context) (models.Feedback, []byte, error) {
	var f models.Feedback
	kind, _, _ := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if kind != "multipart/form-data" {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFeedbackJSON)
		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return f, nil, err
		}
		err = json.Unmarshal(raw, &f)
		return f, nil, err
	}
	if c.Request.ContentLength > maxFeedbackMultipart {
		return f, nil, errFeedbackLimit
	}
	if !allowFeedbackImage(time.Now()) {
		return f, nil, errFeedbackBusy
	}
	select {
	case multipartWork <- struct{}{}:
		defer func() { <-multipartWork }()
	default:
		return f, nil, errFeedbackBusy
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFeedbackMultipart)
	reader, err := c.Request.MultipartReader()
	if err != nil {
		return f, nil, err
	}
	var imageBytes []byte
	haveFeedback, haveImage := false, false
	for {
		p, e := reader.NextPart()
		if e == io.EOF {
			break
		}
		if e != nil {
			return f, nil, e
		}
		switch {
		case p.FormName() == "feedback" && !haveFeedback:
			raw, e := readFeedbackBytes(p, maxFeedbackJSON)
			if e != nil {
				return f, nil, e
			}
			if e = json.Unmarshal(raw, &f); e != nil {
				return f, nil, e
			}
			haveFeedback = true
		case p.FormName() == "image" && !haveImage:
			raw, e := readFeedbackBytes(p, maxFeedbackImage)
			if e != nil {
				return f, nil, e
			}
			imageBytes, e = normalizeFeedbackImage(raw)
			if e != nil {
				return f, nil, e
			}
			haveImage = true
		default:
			return f, nil, errors.New("Expected a feedback field and at most one image.")
		}
	}
	if !haveFeedback {
		return f, nil, errors.New("The feedback JSON field is required.")
	}
	// Check the remainder as MIME parsers may stop at the closing boundary.
	_, err = io.Copy(io.Discard, c.Request.Body)
	return f, imageBytes, err
}

func GetFeedbackImage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Query("id"), 10, strconv.IntSize)
	project := models.NormalizeProjectID(c.Query("project"))
	if err != nil || id == 0 || project == "" {
		c.Status(http.StatusNotFound)
		return
	}
	data, err := models.SelectFeedbackImage(uint(id), project)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("Cross-Origin-Resource-Policy", "same-origin")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", `inline; filename="feedback-screenshot.jpg"`)
	c.Data(http.StatusOK, "image/jpeg", data)
}

type discordImageContext struct {
	ID          string                   `json:"id"`
	ChannelID   string                   `json:"channel_id"`
	Attachments []discordImageAttachment `json:"attachments"`
}

type discordImageAttachment struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

func discordImageURL(a discordImageAttachment, channel string) (*url.URL, bool) {
	if !validSnowflake(a.ID) || !validSnowflake(channel) || a.Size < 0 || a.Size > maxFeedbackImage || len(a.URL) > 2048 {
		return nil, false
	}
	if a.ContentType != "" && a.ContentType != "image/png" && a.ContentType != "image/jpeg" {
		return nil, false
	}
	u, err := url.Parse(a.URL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" ||
		(u.Host != "cdn.discordapp.com" && u.Host != "media.discordapp.net") {
		return nil, false
	}
	parts := strings.Split(u.Path, "/")
	if len(parts) != 5 || parts[1] != "attachments" || parts[2] != channel || parts[3] != a.ID || parts[4] == "" || parts[4] == "." || parts[4] == ".." || strings.Contains(parts[4], "\\") {
		return nil, false
	}
	return u, true
}

var discordScreenshotClient = &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{MaxIdleConnsPerHost: 2, MaxConnsPerHost: 3, DisableCompression: true}}

func downloadDiscordScreenshot(ctx context.Context, client *http.Client, u *url.URL) []byte {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil
	}
	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.ContentLength > maxFeedbackImage {
		return nil
	}
	raw, err := readFeedbackBytes(res.Body, maxFeedbackImage)
	if err != nil {
		return nil
	}
	data, _ := normalizeFeedbackImage(raw)
	return data
}

func fetchDiscordScreenshot(ctx context.Context, s discordSnapshot) []byte {
	select {
	case discordImageWork <- struct{}{}:
		defer func() { <-discordImageWork }()
	default:
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// A flag on a reply uses that reply's attachments, not unrelated starter
	// images. A starter flag or standalone message uses its own attachments.
	attachments, channel := s.Attachments, s.ChannelID
	if s.Trigger.ID != "" && s.Trigger.ID != s.ID {
		attachments, channel = s.Trigger.Attachments, s.Trigger.ChannelID
		if s.Kind == "forum_thread" {
			channel = s.ChannelID
			// The deployed bot carries reply attachments in the forum context.
			if len(attachments) == 0 {
				for _, reply := range s.ThreadContext {
					if reply.ID == s.Trigger.ID {
						attachments = reply.Attachments
						break
					}
				}
			}
		}
	}
	attempts := 0
	for _, a := range attachments {
		u, ok := discordImageURL(a, channel)
		if !ok {
			continue
		}
		attempts++
		if data := downloadDiscordScreenshot(ctx, discordScreenshotClient, u); len(data) > 0 {
			return data
		}
		if attempts == 3 || ctx.Err() != nil {
			break
		}
	}
	return nil
}
