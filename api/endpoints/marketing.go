package endpoints

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"sort"
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
	registerMarketingHandlers(router.Group("/marketing/:project", AdminAuth, func(c *gin.Context) {
		if account, ok := portalAccount(c); ok && account.Role != "admin" {
			c.AbortWithStatusJSON(403, gin.H{"error": "An administrator account is required."})
			return
		}
		c.Next()
	}, marketingScope))
}

func marketingScope(c *gin.Context) {
	if c.Param("project") != marketingProject {
		c.AbortWithStatusJSON(404, gin.H{"error": "Marketing is available for Cursemark."})
		return
	}
	if marketingMCP(c) && c.Query("after_id") != "" {
		if _, err := strconv.ParseUint(c.Query("after_id"), 10, 32); err != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": "Use a valid numeric cursor."})
			return
		}
	}
	c.Header("Cache-Control", "no-store")
	c.Next()
}

func registerMarketingHandlers(g *gin.RouterGroup) {
	g.GET("/beats", listMarketingBeats)
	g.POST("/beats", saveMarketingBeat)
	g.GET("/beats/:beat", getMarketingBeat)
	g.PUT("/beats/:beat", saveMarketingBeat)
	g.PUT("/beats/:beat/archive", archiveMarketingBeat)
	g.POST("/beats/:beat/placements", saveMarketingPlacement)
	g.GET("/beats/:beat/placements/:placement", getMarketingPlacement)
	g.PUT("/beats/:beat/placements/:placement", saveMarketingPlacement)
	g.PUT("/beats/:beat/placements/:placement/archive", archiveMarketingPlacement)
	g.PUT("/beats/:beat/targets/:target", setMarketingBeatTarget)
	g.DELETE("/beats/:beat/targets/:target", setMarketingBeatTarget)
	g.GET("/targets", listMarketingTargets)
	g.POST("/targets", saveMarketingTarget)
	g.POST("/targets/import-creators", func(c *gin.Context) {
		if marketingMCP(c) {
			marketingError(c, 404, "Use the authenticated portal to bulk import private creator contacts.")
			return
		}
		importMarketingCreators(c)
	})
	g.GET("/targets/:target", getMarketingTarget)
	g.PUT("/targets/:target", saveMarketingTarget)
	g.PUT("/targets/:target/archive", archiveMarketingTarget)
	g.PUT("/targets/:target/research", saveMarketingResearch)
	g.PUT("/targets/:target/creator-evidence", saveCreatorEvidence)
	g.GET("/reports", listMarketingReports)
	g.POST("/reports", importMarketingReport)
}

func marketingToken() string {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		panic("marketing identifier generation failed")
	}
	return hex.EncodeToString(token[:])
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

type marketingBeatDraft struct {
	Name         string `json:"name"`
	Brief        string `json:"brief"`
	CallToAction string `json:"call_to_action"`
}

type marketingBeatView struct {
	ID         uint       `json:"id"`
	Version    uint       `json:"version"`
	ArchivedAt *time.Time `json:"archived_at"`
	marketingBeatDraft
}

func (b marketingBeatDraft) validCTA() bool {
	return b.CallToAction == "wishlist" || b.CallToAction == "purchase"
}

func marketingBeatDetails(b models.MarketingBeat) marketingBeatView {
	detail := marketingBeatView{ID: b.ID, Version: b.Version, ArchivedAt: b.ArchivedAt, marketingBeatDraft: marketingBeatDraft{
		Name: b.Name, Brief: b.Brief, CallToAction: b.CallToAction,
	}}
	if strings.TrimSpace(detail.Brief) == "" {
		detail.Brief = b.Hook
	}
	if detail.CallToAction == "" {
		// Display a clear legacy CTA without rewriting the original wording.
		var wish, buy bool
		for _, word := range strings.FieldsFunc(strings.ToLower(b.CTA), func(r rune) bool { return !unicode.IsLetter(r) }) {
			switch word {
			case "wishlist", "wishlists":
				wish = true
			case "buy", "purchase", "purchases":
				buy = true
			}
		}
		if wish && !buy {
			detail.CallToAction = "wishlist"
		}
		if buy && !wish {
			detail.CallToAction = "purchase"
		}
	}
	return detail
}

func readMarketingBeat(db *gorm.DB, id uint) (models.MarketingBeat, error) {
	var b models.MarketingBeat
	err := db.Where("project = ?", marketingProject).First(&b, id).Error
	return b, err
}

func listMarketingBeats(c *gin.Context) {
	var beats []models.MarketingBeat
	if err := marketingListQuery(c, "id DESC").Find(&beats).Error; err != nil {
		marketingDBError(c, err)
		return
	}
	list := make([]marketingBeatView, 0, len(beats))
	for _, beat := range beats {
		list = append(list, marketingBeatDetails(beat))
	}
	c.JSON(200, marketingListResult(c, "beats", list))
}

func saveMarketingBeat(c *gin.Context) {
	var in struct {
		marketingBeatDraft
		Version uint `json:"version"`
	}
	if !marketingJSON(c, &in) {
		return
	}
	if !marketingStrings(map[*string]int{&in.Name: 160, &in.Brief: 3000}) || in.Name == "" || in.Brief == "" || !in.validCTA() {
		marketingError(c, 400, "Enter a Beat name and brief, then choose Wishlist or Purchase.")
		return
	}
	db := marketingDB(c)
	out := models.MarketingBeat{Project: marketingProject, Version: 1, CallToAction: in.CallToAction,
		MarketingBrief: models.MarketingBrief{Name: in.Name, Brief: in.Brief}}
	if c.Request.Method == "POST" {
		if err := db.Create(&out).Error; err != nil {
			marketingDBError(c, err)
			return
		}
		c.JSON(201, marketingBeatDetails(out))
		return
	}
	id, ok := marketingID(c, "beat")
	if !ok {
		return
	}
	update := db.Model(&models.MarketingBeat{}).Where("id = ? AND project = ? AND version = ?", id, marketingProject, in.Version).
		Select("Name", "Brief", "CallToAction", "Version", "UpdatedAt").
		Updates(models.MarketingBeat{MarketingBrief: out.MarketingBrief, CallToAction: in.CallToAction, Version: in.Version + 1})
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
	c.JSON(200, marketingBeatDetails(out))
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
	posts, result, err := marketingResults(db, b.ID)
	if err != nil {
		marketingDBError(c, err)
		return
	}
	ids := []uint{}
	if err := db.Model(&models.MarketingBeatTarget{}).Where("beat_id = ?", id).Order("target_id").Pluck("target_id", &ids).Error; err != nil {
		marketingDBError(c, err)
		return
	}
	out := gin.H{"beat": marketingBeatDetails(b), "placements": posts, "result": result, "target_ids": ids}
	if marketingMCP(c) {
		after, err := strconv.ParseUint(c.DefaultQuery("after_id", "0"), 10, 32)
		if err != nil {
			marketingError(c, 400, "Use a valid placement cursor.")
			return
		}
		sort.Slice(posts, func(i, j int) bool { return posts[i].ID < posts[j].ID })
		start := sort.Search(len(posts), func(i int) bool { return uint64(posts[i].ID) > after })
		page := marketingListResult(c, "placements", posts[start:])
		out["placements"] = page["placements"]
		out["has_more_placements"] = page["has_more"]
		out["placements_limit"] = page["limit"]
	}
	c.JSON(200, out)
}

func getMarketingPlacement(c *gin.Context) {
	beat, ok := marketingID(c, "beat")
	if !ok {
		return
	}
	id, ok := marketingID(c, "placement")
	if !ok {
		return
	}
	db := marketingDB(c)
	if _, err := readMarketingBeat(db, beat); err != nil {
		marketingDBError(c, err)
		return
	}
	posts, _, err := marketingResults(db, beat)
	if err != nil {
		marketingDBError(c, err)
		return
	}
	for _, post := range posts {
		if post.ID == id {
			c.JSON(200, post)
			return
		}
	}
	marketingError(c, 404, "That placement was not found for this Beat.")
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
		TargetID      *uint      `json:"target_id"`
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
	beat, err := readMarketingBeat(db, id)
	if err != nil {
		marketingDBError(c, err)
		return
	}
	p := models.MarketingPlacement{BeatID: id, Project: marketingProject, Version: 1,
		MarketingPlacementDraft: in.MarketingPlacementDraft, PublishedAt: in.PublishedAt,
		PublishedURL: in.PublishedURL, PublishedNote: in.PublishedNote}
	if c.Request.Method == "POST" {
		if beat.ArchivedAt != nil {
			marketingError(c, 409, "Restore the Beat before adding a draft.")
			return
		}
		p.TargetID = in.TargetID
		p.UTM = marketingToken()
		err = db.Transaction(func(tx *gorm.DB) error {
			if in.TargetID != nil {
				var target models.MarketingTarget
				if err := tx.Where("id = ? AND project = ? AND archived_at IS NULL", *in.TargetID, marketingProject).Take(&target).Error; err != nil {
					return err
				}
				var link models.MarketingBeatTarget
				if err := tx.Where("beat_id = ? AND target_id = ?", id, target.ID).Take(&link).Error; err != nil {
					return err
				}
			}
			return tx.Create(&p).Error
		})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			marketingError(c, 409, "Attach an active target before adding a linked draft.")
			return
		}
		if err != nil {
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
