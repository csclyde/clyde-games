package endpoints

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const marketingProject = "cursemark"
const marketingDay = "2006-01-02"

func RegisterMarketingRoutes(router *gin.Engine) {
	g := router.Group("/marketing/:project", AdminAuth, func(c *gin.Context) {
		if c.Param("project") != marketingProject {
			c.AbortWithStatusJSON(404, gin.H{"error": "Marketing is available for Cursemark."})
			return
		}
		if account, ok := portalAccount(c); ok && account.Role != "admin" {
			c.AbortWithStatusJSON(403, gin.H{"error": "An administrator account is required."})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Next()
	})
	g.GET("/beats", listMarketingBeats)
	g.POST("/beats", saveMarketingBeat)
	g.GET("/beats/:beat", getMarketingBeat)
	g.PUT("/beats/:beat", saveMarketingBeat)
	g.POST("/beats/:beat/generate", generateMarketing)
	g.POST("/beats/:beat/placements", saveMarketingPlacement)
	g.PUT("/beats/:beat/placements/:placement", saveMarketingPlacement)
	g.GET("/reports", listMarketingReports)
	g.POST("/reports", importMarketingReport)
}

func marketingDB(c *gin.Context) *gorm.DB {
	return models.AnalyticsDB.WithContext(c.Request.Context()).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}

func marketingError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func marketingDBError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		marketingError(c, 404, "That Beat or placement was not found.")
		return
	}
	marketingError(c, 500, "Marketing data is temporarily unavailable.")
}

func marketingJSON(c *gin.Context, dst any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 48<<10)
	if err := c.ShouldBindJSON(dst); err != nil {
		marketingError(c, 400, "Invalid or oversized marketing form.")
		return false
	}
	return true
}

func marketingID(c *gin.Context, key string) (uint, bool) {
	n, err := strconv.ParseUint(c.Param(key), 10, 32)
	if err != nil || n == 0 {
		marketingError(c, 400, "Invalid "+key+" ID.")
		return 0, false
	}
	return uint(n), true
}

func marketingStrings(limits map[*string]int) bool {
	for field, limit := range limits {
		value := strings.TrimSpace(*field)
		if utf8.RuneCountInString(value) > limit ||
			strings.ContainsRune(value, 0) || !utf8.ValidString(value) {
			return false
		}
		*field = value
	}
	return true
}

func marketingLink(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.User == nil && u.Hostname() != "" &&
		u.Opaque == "" && !strings.ContainsAny(s, "\\<>") && strings.IndexFunc(s, unicode.IsSpace) < 0 &&
		strings.IndexFunc(s, unicode.IsControl) < 0 && len(s) <= 1000
}

func marketingDate(s string) bool {
	t, err := time.Parse(marketingDay, s)
	return err == nil && t.Format(marketingDay) == s
}

func checkMarketingBrief(b *models.MarketingBrief) bool {
	if !marketingStrings(map[*string]int{&b.Name: 160, &b.AssetKind: 24, &b.Asset: 1000, &b.Brief: 3000,
		&b.Hook: 1000, &b.WhyNow: 1000, &b.CTA: 1000, &b.Goal: 24}) || b.Name == "" || b.Asset == "" {
		return false
	}
	switch b.AssetKind {
	case "video", "image", "gif", "trailer", "other":
	default:
		return false
	}
	switch b.Goal {
	case "traffic", "wishlists", "purchases", "activations":
		return b.GoalTarget <= 1000000
	case "demo", "awareness":
		return b.GoalTarget == 0
	}
	return false
}

func readMarketingBeat(db *gorm.DB, id uint) (models.MarketingBeat, error) {
	var b models.MarketingBeat
	err := db.Where("project = ?", marketingProject).First(&b, id).Error
	return b, err
}

func listMarketingBeats(c *gin.Context) {
	var beats []models.MarketingBeat
	if err := marketingDB(c).Where("project = ?", marketingProject).Order("id DESC").Find(&beats).Error; err != nil {
		marketingDBError(c, err)
		return
	}
	c.JSON(200, gin.H{"beats": beats, "ai_enabled": strings.TrimSpace(os.Getenv("CURSEMARK_OPENAI_API_KEY")) != ""})
}

func saveMarketingBeat(c *gin.Context) {
	var in struct {
		models.MarketingBrief
		Version uint `json:"version"`
	}
	if !marketingJSON(c, &in) {
		return
	}
	if !checkMarketingBrief(&in.MarketingBrief) {
		marketingError(c, 400, "Enter a Beat name, one asset and a valid goal. Targets are available for Steam-measured goals.")
		return
	}
	db := marketingDB(c)
	out := models.MarketingBeat{Project: marketingProject, Version: 1, MarketingBrief: in.MarketingBrief}
	if c.Request.Method == "POST" {
		if err := db.Create(&out).Error; err != nil {
			marketingDBError(c, err)
			return
		}
		c.JSON(201, out)
		return
	}
	id, ok := marketingID(c, "beat")
	if !ok {
		return
	}
	update := db.Model(&models.MarketingBeat{}).Where("id = ? AND project = ? AND version = ?", id, marketingProject, in.Version).
		Select("Name", "AssetKind", "Asset", "Brief", "Hook", "WhyNow", "CTA", "Goal", "GoalTarget", "Version", "UpdatedAt").
		Updates(models.MarketingBeat{MarketingBrief: in.MarketingBrief, Version: in.Version + 1})
	if update.Error != nil {
		marketingDBError(c, update.Error)
		return
	}
	if update.RowsAffected != 1 {
		marketingError(c, 409, "This Beat changed. Copy your edits and reload before saving.")
		return
	}
	out, err := readMarketingBeat(db, id)
	if err != nil {
		marketingDBError(c, err)
		return
	}
	c.JSON(200, out)
}

func marketingUTM(p models.MarketingPlacement) string {
	params := url.Values{"utm_source": {"clydegames"}, "utm_medium": {"community"},
		"utm_campaign": {marketingCampaign(p.BeatID)}, "utm_content": {p.UTM}}
	return "https://store.steampowered.com/app/3219180/?" + params.Encode()
}

func marketingCampaign(id uint) string { return "cursemark-beat-" + strconv.FormatUint(uint64(id), 10) }

type marketingPost struct {
	models.MarketingPlacement
	SteamURL string           `json:"steam_url"`
	Result   marketingSummary `json:"result"`
}

func getMarketingBeat(c *gin.Context) {
	id, ok := marketingID(c, "beat")
	if !ok {
		return
	}
	db := marketingDB(c)
	b, err := readMarketingBeat(db, id)
	if err != nil {
		marketingDBError(c, err)
		return
	}
	posts, result, err := marketingResults(db, b)
	if err != nil {
		marketingDBError(c, err)
		return
	}
	c.JSON(200, gin.H{"beat": b, "placements": posts, "result": result})
}

func checkMarketingDraft(d *models.MarketingPlacementDraft) bool {
	if !marketingStrings(map[*string]int{&d.Platform: 80, &d.Community: 160, &d.AudienceFit: 1500, &d.Timing: 1000,
		&d.Title: 255, &d.Copy: 5000, &d.AssetNotes: 1500, &d.RuleEvidence: 2500, &d.RulesURL: 1000,
		&d.RulesChecked: 10, &d.Decision: 16}) || d.Platform == "" || d.Community == "" {
		return false
	}
	if (d.RulesURL != "" && !marketingLink(d.RulesURL)) || (d.RulesChecked != "" &&
		(!marketingDate(d.RulesChecked) || d.RulesChecked > time.Now().UTC().Format(marketingDay))) {
		return false
	}
	return d.Decision == "hold" || (d.Decision == "candidate" && d.RulesURL != "" && d.RulesChecked != "" && d.RuleEvidence != "")
}

func saveMarketingPlacement(c *gin.Context) {
	id, ok := marketingID(c, "beat")
	if !ok {
		return
	}
	var in struct {
		models.MarketingPlacementDraft
		Version       uint       `json:"version"`
		PublishedAt   *time.Time `json:"published_at"`
		PublishedURL  string     `json:"published_url"`
		PublishedNote string     `json:"published_note"`
	}
	if !marketingJSON(c, &in) {
		return
	}
	in.PublishedURL = strings.TrimSpace(in.PublishedURL)
	in.PublishedNote = strings.TrimSpace(in.PublishedNote)
	hasPublication := in.PublishedAt != nil
	if !checkMarketingDraft(&in.MarketingPlacementDraft) || hasPublication != (in.PublishedURL != "") ||
		(hasPublication && (!marketingLink(in.PublishedURL) || in.PublishedAt.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) ||
			in.PublishedAt.After(time.Now()))) || utf8.RuneCountInString(in.PublishedNote) > 3000 {
		marketingError(c, 400, "Enter a platform and community. A candidate needs dated rule evidence. Recording publication requires a live HTTPS URL and a past date.")
		return
	}
	db := marketingDB(c)
	if _, err := readMarketingBeat(db, id); err != nil {
		marketingDBError(c, err)
		return
	}
	p := models.MarketingPlacement{BeatID: id, Project: marketingProject, Version: 1,
		MarketingPlacementDraft: in.MarketingPlacementDraft, PublishedAt: in.PublishedAt,
		PublishedURL: in.PublishedURL, PublishedNote: in.PublishedNote}
	if c.Request.Method == "POST" {
		p.UTM = marketingToken()
		if err := db.Create(&p).Error; err != nil {
			marketingDBError(c, err)
			return
		}
		c.JSON(201, marketingPost{MarketingPlacement: p, SteamURL: marketingUTM(p)})
		return
	}
	placement, ok := marketingID(c, "placement")
	if !ok {
		return
	}
	scope := db.Where("id = ? AND beat_id = ? AND project = ?", placement, id, marketingProject)
	var prior models.MarketingPlacement
	if err := scope.First(&prior).Error; err != nil {
		marketingDBError(c, err)
		return
	}
	p.Version = in.Version + 1
	p.SourceCited = prior.SourceCited && p.RulesURL == prior.RulesURL && p.RulesChecked == prior.RulesChecked && p.RuleEvidence == prior.RuleEvidence &&
		p.Platform == prior.Platform && p.Community == prior.Community
	result := db.Model(&models.MarketingPlacement{}).Where("id = ? AND beat_id = ? AND project = ? AND version = ?", placement, id, marketingProject, in.Version).
		Select("Platform", "Community", "AudienceFit", "Timing", "Title", "Copy", "AssetNotes", "RuleEvidence", "RulesURL",
			"RulesChecked", "Decision", "Version", "SourceCited", "PublishedAt", "PublishedURL", "PublishedNote").Updates(p)
	if result.Error != nil {
		marketingDBError(c, result.Error)
		return
	}
	if result.RowsAffected != 1 {
		marketingError(c, 409, "This placement changed. Copy your edits and reload before saving.")
		return
	}
	if err := scope.First(&p).Error; err != nil {
		marketingDBError(c, err)
		return
	}
	c.JSON(200, marketingPost{MarketingPlacement: p, SteamURL: marketingUTM(p)})
}
