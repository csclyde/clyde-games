package endpoints

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type marketingCreatorFields struct {
	Email       string   `json:"email"`
	Platform    string   `json:"platform"`
	ChannelURLs []string `json:"channel_urls"`
	Games       []string `json:"games_covered"`
}

type marketingTargetInput struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Kind    string `json:"kind"`
	Version uint   `json:"version"`
	marketingCreatorFields
}

var contactPattern = regexp.MustCompile("(?i)[a-z0-9.!#$%&'*+/=^_`{|}~-]+@[a-z0-9-]+(?:\\.[a-z0-9-]+)+")
var twitchChannel = regexp.MustCompile("^[a-zA-Z0-9_]{2,30}$")

const creatorResearchInstructions = `Research the supplied public creator channel URLs for potential fit with Cursemark. Search those channels only. Cite the actual channel and up to four recent dated videos, clips, or articles visibly authored by the creator, with exact links and visible engagement when available. Return titles of games verified by the cited recent coverage in games_covered. Keep game titles and coverage evidence separate from inference about fit. Do not search for or return names supplied outside these channels, contact details, emails, correspondence, private profiles, or guessed industry relationships. Never claim an invitation or consent to contact. Return empty rule fields and ai_restriction. If public coverage cannot be verified, say so and set hold=true. Use plain text.`

func safeResearchURL(link string) bool {
	if !marketingLink(link) {
		return false
	}
	decoded, err := url.PathUnescape(link)
	return err == nil && !contactPattern.MatchString(decoded)
}

func withoutContact(value string) string {
	return contactPattern.ReplaceAllString(value, "[contact omitted]")
}

func cleanResearchText(r *marketingResearch) {
	for _, field := range []*string{&r.Takeaway, &r.HoldReason, &r.Rules.Links, &r.Rules.SelfPromotion, &r.Rules.AI, &r.Rules.Flair, &r.Rules.Cadence} {
		*field = withoutContact(*field)
	}
	for i := range r.Examples {
		ex := &r.Examples[i]
		ex.Title = withoutContact(ex.Title)
		ex.Engagement = withoutContact(ex.Engagement)
		ex.WhatWorked = withoutContact(ex.WhatWorked)
	}
	sources := []string{}
	for _, source := range r.Sources {
		if safeResearchURL(source) {
			sources = append(sources, source)
		}
	}
	r.Sources = sources
	examples := []marketingExample{}
	for _, example := range r.Examples {
		if safeResearchURL(example.URL) {
			examples = append(examples, example)
		}
	}
	r.Examples = examples
	games := []string{}
	for _, game := range r.Games {
		if !contactPattern.MatchString(game) && marketingStrings(map[*string]int{&game: 160}) && game != "" && len(games) < 15 {
			games = append(games, game)
		}
	}
	r.Games = games
}

func creatorEvidence(channels []string, source string, official bool) bool {
	if !safeResearchURL(source) {
		return false
	}
	s, _ := url.Parse(source)
	for _, channel := range channels {
		c, _ := url.Parse(channel)
		host := strings.ToLower(s.Hostname())
		if official {
			if marketingSourceKey(channel) == marketingSourceKey(source) ||
				(host == strings.ToLower(c.Hostname()) && c.Path != "" && strings.HasPrefix(s.Path, c.Path+"/")) {
				return true
			}
			continue
		}
		switch {
		case domain(c.Hostname(), "youtube.com") && domain(host, "youtube.com"):
			if s.Path == "/watch" && s.Query().Get("v") != "" || strings.HasPrefix(s.Path, "/shorts/") {
				return true
			}
		case domain(c.Hostname(), "twitch.tv") && domain(host, "twitch.tv"):
			if strings.HasPrefix(s.Path, "/videos/") || strings.HasPrefix(s.Path, c.Path+"/clip/") {
				return true
			}
		default:
			if host == strings.ToLower(c.Hostname()) && s.Path != c.Path && strings.HasPrefix(s.Path, strings.TrimRight(c.Path, "/")+"/") {
				return true
			}
		}
	}
	return false
}

func creatorChannel(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !marketingLink(value) {
		return "", false
	}
	u, _ := url.Parse(value)
	host := strings.ToLower(u.Hostname())
	if u.Port() != "" || net.ParseIP(host) != nil || !strings.Contains(host, ".") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".localhost") ||
		contactPattern.MatchString(host+u.Path) {
		return "", false
	}
	u.Host = host
	u.Fragment = ""
	u.RawQuery = ""
	u.ForceQuery = false
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch {
	case domain(host, "youtube.com"):
		if !((len(parts) == 1 && strings.HasPrefix(parts[0], "@") && len(parts[0]) > 1) ||
			(len(parts) == 2 && (parts[0] == "channel" || parts[0] == "c" || parts[0] == "user") && parts[1] != "")) {
			return "", false
		}
		u.Host = "www.youtube.com"
	case domain(host, "twitch.tv"):
		if len(parts) != 1 || !twitchChannel.MatchString(parts[0]) || parts[0] == "videos" {
			return "", false
		}
		u.Host = "www.twitch.tv"
	case domain(host, "reddit.com"):
		if len(parts) != 2 || !strings.EqualFold(parts[0], "user") || !subredditName.MatchString(parts[1]) {
			return "", false
		}
	case domain(host, "x.com"), domain(host, "twitter.com"):
		if len(parts) != 1 || parts[0] == "" {
			return "", false
		}
		u.Host = "x.com"
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	return u.String(), len(u.String()) <= 1000
}

func prepareCreator(input marketingTargetInput) (models.MarketingTarget, models.MarketingCreator, error) {
	var target models.MarketingTarget
	var person models.MarketingCreator
	if !marketingStrings(map[*string]int{&input.Name: 160, &input.Email: 254}) || input.Name == "" {
		return target, person, errors.New("a creator name and contact address are required")
	}
	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Name != "" || address.Address != input.Email || !contactPattern.MatchString(input.Email) {
		return target, person, errors.New("the contact address must be a single valid email")
	}
	if input.Platform != "youtube" && input.Platform != "twitch" && input.Platform != "other" {
		return target, person, errors.New("choose YouTube, Twitch or Other for the creator platform")
	}
	if len(input.ChannelURLs) > 6 || len(input.Games) > 30 {
		return target, person, errors.New("use at most six public channels and 30 games")
	}
	links := []string{}
	seen := map[string]bool{}
	for _, item := range input.ChannelURLs {
		if strings.TrimSpace(item) == "" {
			continue
		}
		channel, ok := creatorChannel(item)
		if !ok {
			return target, person, errors.New("enter only public HTTPS channel URLs")
		}
		key := marketingSourceKey(channel)
		if !seen[key] {
			links = append(links, channel)
			seen[key] = true
		}
	}
	games := []string{}
	seen = map[string]bool{}
	for _, game := range input.Games {
		if !marketingStrings(map[*string]int{&game: 160}) {
			return target, person, errors.New("a game title is too long")
		}
		key := strings.ToLower(game)
		if game != "" && !seen[key] {
			games = append(games, game)
			seen[key] = true
		}
	}
	encodedLinks, _ := json.Marshal(links)
	encodedGames, _ := json.Marshal(games)
	key := sha256.Sum256([]byte("creator\x00" + strings.ToLower(input.Email)))
	target = models.MarketingTarget{Project: marketingProject, Name: input.Name, URLKey: hex.EncodeToString(key[:]), Version: 1, IsCreator: true}
	if len(links) > 0 {
		target.URL = links[0]
	}
	person = models.MarketingCreator{Email: input.Email, Platform: input.Platform, ChannelURLs: string(encodedLinks), Games: string(encodedGames)}
	return target, person, nil
}

func readMarketingCreator(db *gorm.DB, targetID uint) (models.MarketingCreator, error) {
	var creator models.MarketingCreator
	err := db.First(&creator, targetID).Error
	return creator, err
}

func creatorFields(c models.MarketingCreator) marketingCreatorFields {
	out := marketingCreatorFields{Email: c.Email, Platform: c.Platform, ChannelURLs: []string{}, Games: []string{}}
	_ = json.Unmarshal([]byte(c.ChannelURLs), &out.ChannelURLs)
	_ = json.Unmarshal([]byte(c.Games), &out.Games)
	return out
}

func creatorResearchKey(c models.MarketingCreator) string {
	value := sha256.Sum256([]byte(c.Platform + "\x00" + c.ChannelURLs))
	return "creator:" + hex.EncodeToString(value[:])
}

var errCreatorVersion = errors.New("creator contact version changed")

func saveMarketingCreator(c *gin.Context, in marketingTargetInput) {
	target, person, err := prepareCreator(in)
	if err != nil {
		marketingError(c, 400, err.Error())
		return
	}
	db := marketingDB(c)
	status := 201
	if c.Request.Method == http.MethodPut {
		id, ok := marketingID(c, "target")
		if !ok {
			return
		}
		target.ID = id
		status = 200
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if status == 201 {
			if err := tx.Create(&target).Error; err != nil {
				return err
			}
			person.TargetID = target.ID
			return tx.Create(&person).Error
		}
		update := tx.Model(&models.MarketingTarget{}).
			Where("id = ? AND project = ? AND version = ? AND is_creator = ?", target.ID, marketingProject, in.Version, true).
			Updates(map[string]any{"name": target.Name, "url": target.URL, "url_key": target.URLKey, "version": in.Version + 1})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errCreatorVersion
		}
		return tx.Model(&models.MarketingCreator{}).Where("target_id = ?", target.ID).
			Updates(map[string]any{"email": person.Email, "platform": person.Platform, "channel_urls": person.ChannelURLs, "games": person.Games}).Error
	})
	if errors.Is(err, errCreatorVersion) {
		marketingError(c, 409, "This creator changed. Reload before saving.")
		return
	}
	if err != nil {
		marketingTargetError(c, err)
		return
	}
	target, err = readTarget(db, target.ID)
	if err != nil {
		marketingTargetError(c, err)
		return
	}
	out, err := targetPrivateDetails(db, target)
	if err != nil {
		marketingDBError(c, err)
		return
	}
	c.JSON(status, out)
}

type creatorImportRow struct {
	target  models.MarketingTarget
	creator models.MarketingCreator
}

func readCreatorCSV(input io.Reader) ([]creatorImportRow, error) {
	r := csv.NewReader(input)
	head, err := r.Read()
	if err != nil {
		return nil, errors.New("choose a valid creator CSV")
	}
	columns := map[string]int{}
	for i, name := range head {
		columns[strings.TrimSpace(strings.TrimPrefix(strings.ToLower(name), "\ufeff"))] = i
	}
	fields := []string{"name", "email", "platform", "channel_urls", "games_covered"}
	if len(columns) != len(fields) || len(head) != len(fields) {
		return nil, errors.New("the CSV requires name, email, platform, channel_urls and games_covered")
	}
	for _, key := range fields {
		if _, ok := columns[key]; !ok {
			return nil, errors.New("the CSV requires name, email, platform, channel_urls and games_covered")
		}
	}
	out := []creatorImportRow{}
	for line := 2; ; line++ {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || line > 201 {
			return nil, fmt.Errorf("invalid CSV near row %d or over 200 creators", line)
		}
		field := func(key string) string { return row[columns[key]] }
		target, person, err := prepareCreator(marketingTargetInput{Name: field("name"), Kind: "creator", marketingCreatorFields: marketingCreatorFields{
			Email: field("email"), Platform: strings.ToLower(strings.TrimSpace(field("platform"))),
			ChannelURLs: strings.Split(field("channel_urls"), "|"), Games: strings.Split(field("games_covered"), "|"),
		}})
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", line, err)
		}
		out = append(out, creatorImportRow{target, person})
	}
	if len(out) == 0 {
		return nil, errors.New("the file has no creator rows")
	}
	return out, nil
}

func importMarketingCreators(c *gin.Context) {
	const limit = 512 << 10
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit+(16<<10))
	if err := c.Request.ParseMultipartForm(limit + (16 << 10)); err != nil {
		marketingError(c, 400, "Upload a creator CSV up to 512 KB.")
		return
	}
	defer c.Request.MultipartForm.RemoveAll()
	file, header, err := c.Request.FormFile("csv")
	if err != nil {
		marketingError(c, 400, "Choose a creator CSV.")
		return
	}
	defer file.Close()
	if header.Size > limit {
		marketingError(c, 400, "Upload a creator CSV up to 512 KB.")
		return
	}
	rows, err := readCreatorCSV(io.LimitReader(file, limit))
	if err != nil {
		marketingError(c, 400, err.Error())
		return
	}
	added := 0
	err = marketingDB(c).Transaction(func(tx *gorm.DB) error {
		for _, row := range rows {
			created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row.target)
			if created.Error != nil {
				return created.Error
			}
			if created.RowsAffected == 0 {
				continue
			}
			row.creator.TargetID = row.target.ID
			if err := tx.Create(&row.creator).Error; err != nil {
				return err
			}
			added++
		}
		return nil
	})
	if err != nil {
		marketingDBError(c, err)
		return
	}
	c.JSON(200, gin.H{"added": added, "duplicates": len(rows) - added})
}
