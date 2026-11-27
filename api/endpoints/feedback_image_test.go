package endpoints

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func imageFixture(t *testing.T, w, h int) []byte {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	im.Set(1, 1, color.RGBA{R: 255, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func testImageRoute(r *gin.Engine, method, path, token, contentType string, data []byte) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Admin-API-Token", token)
	r.ServeHTTP(w, req)
	return w
}

func multipartImage(t *testing.T, metadata string, files ...[]byte) ([]byte, string) {
	t.Helper()
	var raw bytes.Buffer
	m := multipart.NewWriter(&raw)
	if err := m.WriteField("feedback", metadata); err != nil {
		t.Fatal(err)
	}
	for _, data := range files {
		p, err := m.CreateFormFile("image", "ignored.svg")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = p.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	return raw.Bytes(), m.FormDataContentType()
}

func TestFeedbackImageUploadRejectsBeforeDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/feedback", AddFeedback)
	picture := imageFixture(t, 3, 3)
	cases := []struct {
		files [][]byte
		want  int
	}{
		{[][]byte{[]byte("<svg/>")}, 415},
		{[][]byte{picture[:len(picture)/2]}, 415},
		{[][]byte{imageFixture(t, 4097, 2)}, 415},
		{[][]byte{make([]byte, maxFeedbackImage+1)}, 413},
		{[][]byte{picture, picture}, 400},
	}
	for _, tc := range cases {
		raw, ct := multipartImage(t, `{"Project":"cursemark","Message":"test"}`, tc.files...)
		w := testImageRoute(r, "POST", "/feedback", "", ct, raw)
		if w.Code != tc.want {
			t.Fatalf("%d != %d: %s", w.Code, tc.want, w.Body)
		}
	}
	raw := bytes.Repeat([]byte{' '}, maxFeedbackJSON+1)
	if w := testImageRoute(r, "POST", "/feedback", "", "application/json", raw); w.Code != 413 {
		t.Fatal(w.Code, w.Body)
	}
}

func TestFeedbackImageMySQLGameAndClose(t *testing.T) {
	db := syncTestDB(t)
	if err := db.AutoMigrate(&models.Savegame{}, &models.FeedbackImage{}); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 40)
	t.Setenv("ADMIN_API_TOKEN", token)
	t.Setenv("PORTAL_AUTH_MODE", "legacy")
	r := gin.New()
	r.POST("/feedback", AddFeedback)
	r.GET("/feedback", AdminAuth, GetFeedback)
	r.GET("/feedback/image", AdminAuth, GetFeedbackImage)
	r.POST("/close", AdminAuth, CloseFeedback)
	r.POST("/legacy", AdminAuth, ResolveFeedback)
	const meta = `{"Project":"cursemark","PID":"screen-fixture","Message":"keep after close","HasImage":true}`
	getCreated := func(w *httptest.ResponseRecorder, has bool) models.Feedback {
		t.Helper()
		var f models.Feedback
		if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &f) != nil || f.ID == 0 || f.HasImage != has {
			t.Fatalf("create %d %s", w.Code, w.Body)
		}
		return f
	}
	getCreated(testImageRoute(r, "POST", "/feedback", "", "application/json", []byte(meta)), false)
	raw, ct := multipartImage(t, meta)
	getCreated(testImageRoute(r, "POST", "/feedback", "", ct, raw), false)
	raw, ct = multipartImage(t, meta, imageFixture(t, 6, 4))
	for _, disposition := range []string{"resolved", "ignored", "legacy"} {
		f := getCreated(testImageRoute(r, "POST", "/feedback", "", ct, raw), true)
		id := strconv.FormatUint(uint64(f.ID), 10)
		p := "/feedback/image?id=" + id + "&project="
		for _, tc := range []struct {
			path, auth string
			want       int
		}{{p + "cursemark", "", 401}, {p + "necrovale", token, 404}, {p + "cursemark", token, 200}} {
			w := testImageRoute(r, "GET", tc.path, tc.auth, "", nil)
			if w.Code != tc.want {
				t.Fatalf("%d != %d", w.Code, tc.want)
			}
			if tc.want == 200 {
				conf, format, err := image.DecodeConfig(w.Body)
				if err != nil || format != "jpeg" || conf.Width != 6 || conf.Height != 4 || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
					t.Fatal("invalid image response", err)
				}
			}
		}
		w := testImageRoute(r, "GET", "/feedback", token, "", nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"HasImage": true`) {
			t.Fatal(w.Code, w.Body)
		}
		endpoint := "/close"
		if disposition == "legacy" {
			endpoint = "/legacy"
		}
		w = testImageRoute(r, "POST", endpoint+"?id="+id, token, "application/json", []byte(`{"disposition":"`+disposition+`"}`))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
		var n int64
		if db.Model(&models.FeedbackImage{}).Where("feedback_id = ?", f.ID).Count(&n).Error != nil || n != 0 {
			t.Fatal("image not deleted", n)
		}
		if err := db.First(&f, f.ID).Error; err != nil || f.HasImage || !f.Resolved || f.Message != "keep after close" {
			t.Fatal(f, err)
		}
		if w = testImageRoute(r, "GET", p+"cursemark", token, "", nil); w.Code != 404 {
			t.Fatal(w.Code)
		}
	}
}

type imageRoundTrip func(*http.Request) (*http.Response, error)

func (fn imageRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func TestFeedbackImageMySQLDiscordManual(t *testing.T) {
	r, token := manualFixture(t)
	t.Setenv("DISCORD_MANUAL_DEFAULT_PROJECT", "cursemark")
	if err := models.AnalyticsDB.AutoMigrate(&models.Savegame{}, &models.FeedbackImage{}); err != nil {
		t.Fatal(err)
	}
	picture := imageFixture(t, 4, 3)
	fetches := 0
	original := discordScreenshotClient
	t.Cleanup(func() { discordScreenshotClient = original })
	discordScreenshotClient = &http.Client{Transport: imageRoundTrip(func(r *http.Request) (*http.Response, error) {
		fetches++
		if r.URL.Host != "cdn.discordapp.com" || r.URL.Path != "/attachments/191/193/mock.png" || r.URL.Query().Get("ex") != "signed" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, ContentLength: int64(len(picture)), Body: io.NopCloser(bytes.NewReader(picture))}, nil
	})}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(manualBody("191", "191")), &data); err != nil {
		t.Fatal(err)
	}
	data["kind"] = "forum_thread"
	data["parent_id"] = "190"
	data["scope_id"] = "190"
	data["trigger_message"] = map[string]string{"id": "192", "channel_id": "191"}
	data["thread_context"] = []interface{}{map[string]interface{}{"id": "192", "attachments": []discordImageAttachment{{ID: "193", URL: "https://cdn.discordapp.com/attachments/191/193/mock.png?ex=signed", ContentType: "image/png"}}}}
	offer := func() {
		t.Helper()
		raw, _ := json.Marshal(data)
		if w := manualRequest(r, "POST", "/manual", token, string(raw)); w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	}
	offer()
	offer()
	var saved models.Feedback
	if err := models.AnalyticsDB.Where("source = ?", "discord").First(&saved).Error; err != nil || !saved.HasImage || fetches != 1 {
		t.Fatal(saved, fetches, err)
	}
	if _, err := models.CloseFeedback(strconv.Itoa(int(saved.ID)), models.FeedbackIgnored); err != nil {
		t.Fatal(err)
	}
	offer()
	if fetches != 1 {
		t.Fatal("closed retry downloaded image")
	}
	data["id"] = "195"
	data["channel_id"] = "195"
	data["trigger_message"] = map[string]string{"id": "196"}
	data["thread_context"] = []interface{}{map[string]interface{}{"id": "196", "attachments": []discordImageAttachment{{ID: "197", URL: "https://example.test/attachments/195/197/mock.png", ContentType: "image/png"}}}}
	offer()
	if fetches != 1 {
		t.Fatal("untrusted host downloaded")
	}
	var missing models.Feedback
	if e := models.AnalyticsDB.Where("source_id LIKE ?", "%:195").First(&missing).Error; e != nil || missing.HasImage || missing.Message == "" {
		t.Fatal(missing, e)
	}
	var n int64
	if e := models.AnalyticsDB.Model(&models.FeedbackImage{}).Count(&n).Error; e != nil || n != 0 {
		t.Fatal(n, e)
	}
	if _, err := models.SelectFeedbackImage(saved.ID, saved.Project); err != gorm.ErrRecordNotFound {
		t.Fatal(err)
	}
}

func TestDiscordImageURLAndDownloadLimits(t *testing.T) {
	for _, tc := range []struct {
		url     string
		allowed bool
	}{
		{"https://cdn.discordapp.com/attachments/123/345/file.png?ex=abc", true},
		{"https://media.discordapp.net/attachments/123/345/file.png", true},
		{"http://cdn.discordapp.com/attachments/123/345/file.png", false},
		{"https://cdn.discordapp.com.example.org/attachments/123/345/file.png", false},
		{"https://cdn.discordapp.com@attacker.test/attachments/123/345/file.png", false},
		{"https://cdn.discordapp.com/attachments/321/345/file.png", false},
		{"https://cdn.discordapp.com/attachments/123/543/file.png", false},
	} {
		u, ok := discordImageURL(discordImageAttachment{ID: "345", URL: tc.url}, "123")
		if ok != tc.allowed {
			t.Fatalf("%s allowed=%v", tc.url, ok)
		}
		if !ok {
			continue
		}
		for _, status := range []int{200, 302} {
			client := &http.Client{Transport: imageRoundTrip(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, ContentLength: -1, Body: io.NopCloser(bytes.NewReader(make([]byte, maxFeedbackImage+1)))}, nil
			})}
			if b := downloadDiscordScreenshot(httptest.NewRequest("GET", "/", nil).Context(), client, u); len(b) != 0 {
				t.Fatal("unexpected stored image")
			}
		}
	}
}
