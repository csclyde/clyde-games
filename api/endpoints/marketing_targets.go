package endpoints

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type marketingExample struct {
	Title      string `json:"title"`
	URL        string `json:"url"`
	Date       string `json:"date"`
	Engagement string `json:"engagement"`
	WhatWorked string `json:"what_worked"`
}

type marketingRules struct {
	Links         string `json:"links"`
	SelfPromotion string `json:"self_promotion"`
	AI            string `json:"ai"`
	Flair         string `json:"flair"`
	Cadence       string `json:"cadence"`
}

type marketingResearch struct {
	Rules      marketingRules     `json:"rules"`
	Sources    []string           `json:"sources"`
	Examples   []marketingExample `json:"examples"`
	Takeaway   string             `json:"takeaway"`
	Hold       bool               `json:"hold"`
	HoldReason string             `json:"hold_reason"`
}

type marketingTargetView struct {
	ID          uint               `json:"id"`
	Version     uint               `json:"version"`
	Name        string             `json:"name"`
	URL         string             `json:"url"`
	ResearchURL string             `json:"research_url"`
	CheckedAt   *time.Time         `json:"checked_at"`
	Research    *marketingResearch `json:"research"`
}

var subredditName = regexp.MustCompile("^[a-zA-Z0-9_]{2,30}$")

func targetURL(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !marketingLink(value) {
		return "", false
	}
	u, _ := url.Parse(value)
	host := strings.ToLower(u.Hostname())
	if u.Port() != "" || net.ParseIP(host) != nil || !strings.Contains(host, ".") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".localhost") {
		return "", false
	}
	u.Host = host
	u.Fragment = ""
	q := u.Query()
	for key := range q {
		if strings.HasPrefix(strings.ToLower(key), "utm_") {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()
	u.ForceQuery = false
	if domain(host, "reddit.com") {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "r") || !subredditName.MatchString(parts[1]) {
			return "", false
		}
		return "https://www.reddit.com/r/" + strings.ToLower(parts[1]) + "/", true
	}
	if domain(host, "x.com") || domain(host, "twitter.com") {
		u.Host = "x.com"
	}
	return u.String(), len(u.String()) <= 1000
}

func domain(host, base string) bool { return host == base || strings.HasSuffix(host, "."+base) }

func marketingSourceKey(value string) string {
	u, _ := url.Parse(value)
	if u == nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if domain(host, "reddit.com") {
		host = "reddit.com"
		u.Path = strings.ToLower(u.Path)
	}
	if domain(host, "twitter.com") || domain(host, "x.com") {
		host = "x.com"
	}
	return host + strings.TrimRight(u.Path, "/")
}

func targetDetails(target models.MarketingTarget) marketingTargetView {
	out := marketingTargetView{ID: target.ID, Version: target.Version, Name: target.Name, URL: target.URL,
		ResearchURL: target.ResearchURL, CheckedAt: target.CheckedAt}
	if target.Research != "" {
		var research marketingResearch
		if json.Unmarshal([]byte(target.Research), &research) == nil {
			out.Research = &research
		}
	}
	return out
}

func readTarget(db *gorm.DB, id uint) (models.MarketingTarget, error) {
	var out models.MarketingTarget
	err := db.Where("project = ?", marketingProject).First(&out, id).Error
	return out, err
}

func marketingTargetError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		marketingError(c, 404, "That target or Beat was not found.")
		return
	}
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		marketingError(c, 409, "This destination is already in the target catalog.")
		return
	}
	marketingDBError(c, err)
}

func listMarketingTargets(c *gin.Context) {
	var targets []models.MarketingTarget
	if err := marketingDB(c).Where("project = ?", marketingProject).Order("name ASC, id ASC").Find(&targets).Error; err != nil {
		marketingDBError(c, err)
		return
	}
	out := make([]marketingTargetView, 0, len(targets))
	for _, target := range targets {
		out = append(out, targetDetails(target))
	}
	c.JSON(200, gin.H{"targets": out})
}

func saveMarketingTarget(c *gin.Context) {
	var in struct {
		Name    string `json:"name"`
		URL     string `json:"url"`
		Version uint   `json:"version"`
	}
	if !marketingJSON(c, &in) {
		return
	}
	link, validURL := targetURL(in.URL)
	if !validURL || !marketingStrings(map[*string]int{&in.Name: 160}) || in.Name == "" {
		marketingError(c, 400, "Enter a name and public HTTPS destination. For Reddit, use the subreddit home URL.")
		return
	}
	canonical, _ := url.Parse(link)
	initial := sha256.Sum256([]byte(marketingSourceKey(link) + "?" + canonical.RawQuery))
	db := marketingDB(c)
	target := models.MarketingTarget{Project: marketingProject, Name: in.Name, URL: link, URLKey: hex.EncodeToString(initial[:]), Version: 1}
	if c.Request.Method == "POST" {
		if err := db.Create(&target).Error; err != nil {
			marketingTargetError(c, err)
			return
		}
		c.JSON(201, targetDetails(target))
		return
	}
	id, ok := marketingID(c, "target")
	if !ok {
		return
	}
	update := db.Model(&models.MarketingTarget{}).Where("id = ? AND project = ? AND version = ?", id, marketingProject, in.Version).
		Updates(map[string]any{"name": target.Name, "url": target.URL, "url_key": target.URLKey, "version": in.Version + 1})
	if update.Error != nil {
		marketingTargetError(c, update.Error)
		return
	}
	if update.RowsAffected != 1 {
		marketingError(c, 409, "This target changed. Reload before saving.")
		return
	}
	out, err := readTarget(db, id)
	if err != nil {
		marketingTargetError(c, err)
		return
	}
	c.JSON(200, targetDetails(out))
}

func setMarketingBeatTarget(c *gin.Context) {
	beatID, ok := marketingID(c, "beat")
	if !ok {
		return
	}
	targetID, ok := marketingID(c, "target")
	if !ok {
		return
	}
	link := models.MarketingBeatTarget{BeatID: beatID, TargetID: targetID}
	err := marketingDB(c).Transaction(func(tx *gorm.DB) error {
		if _, err := readMarketingBeat(tx.Clauses(clause.Locking{Strength: "UPDATE"}), beatID); err != nil {
			return err
		}
		if _, err := readTarget(tx, targetID); err != nil {
			return err
		}
		if c.Request.Method == "DELETE" {
			return tx.Where("beat_id = ? AND target_id = ?", beatID, targetID).Delete(&link).Error
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error
	})
	if err != nil {
		marketingTargetError(c, err)
		return
	}
	c.Status(204)
}

const targetResearchInstructions = `Research only the exact public destination URL in the input for Cursemark, a dark fantasy action roguelite. Find up to four genuine posts promoting games from the past twelve months, favoring the latest six months, with exact permalinks on the destination, YYYY-MM-DD dates, and visible engagement. Leave engagement empty when it cannot be read. Distinguish observations from inferences about why a post got engagement. Do not treat past posts as permission. Find the current official rules or moderator guidelines specific to the destination. Summarize each of links, self-promotion, AI-generated content or disclosure, flair, and cadence. Say Unclear when a rule cannot be confirmed. Return actual official sources separately from post links. Hold=true when permission for AI-assisted game promotion is unclear or restricted, requires approval, or has conflicting guidance. Explain the reason. Takeaway should distinguish actionable format patterns from speculation. Do not recommend disguising AI use. Return plain text in string fields.`

func marketingResearchSchema() map[string]any {
	props := marketingTextFields("takeaway", "hold_reason")
	props["hold"] = map[string]string{"type": "boolean"}
	props["sources"] = map[string]any{"type": "array", "items": map[string]string{"type": "string"}}
	props["rules"] = marketingObject(marketingTextFields("links", "self_promotion", "ai", "flair", "cadence"))
	props["examples"] = map[string]any{"type": "array", "items": marketingObject(marketingTextFields("title", "url", "date", "engagement", "what_worked"))}
	return marketingObject(props)
}

func researchLinkMatches(target, source string, official bool) bool {
	if !marketingLink(source) {
		return false
	}
	t, _ := url.Parse(target)
	s, _ := url.Parse(source)
	targetHost, sourceHost := strings.ToLower(t.Hostname()), strings.ToLower(s.Hostname())
	path := strings.TrimRight(strings.ToLower(s.Path), "/")
	if domain(targetHost, "reddit.com") {
		if !domain(sourceHost, "reddit.com") {
			return false
		}
		community := strings.TrimRight(strings.ToLower(t.Path), "/")
		if official {
			return path == community+"/about/rules" || path == "/mod/"+strings.TrimPrefix(community, "/r/")+"/rules" ||
				path == community+"/wiki" || strings.HasPrefix(path, community+"/wiki/")
		}
		return strings.HasPrefix(path, community+"/comments/") && len(strings.TrimPrefix(path, community+"/comments/")) > 0
	}
	if targetHost == "x.com" {
		if official {
			return sourceHost == "help.x.com" || sourceHost == "help.twitter.com" || sourceHost == "x.com" && strings.HasPrefix(path, "/rules")
		}
		if !domain(sourceHost, "x.com") && !domain(sourceHost, "twitter.com") {
			return false
		}
		return strings.Contains(path, "/status/") && (t.Path == "" || t.Path == "/" || strings.HasPrefix(path, strings.TrimRight(strings.ToLower(t.Path), "/")+"/status/"))
	}
	sameHost := sourceHost == targetHost || strings.TrimPrefix(sourceHost, "www.") == strings.TrimPrefix(targetHost, "www.")
	if !sameHost {
		return false
	}
	scope := strings.TrimRight(strings.ToLower(t.Path), "/")
	return (scope == "" || strings.HasPrefix(path, scope+"/")) && path != "" && path != scope
}

func validateResearch(target string, raw []byte, evidence map[string]bool, checked time.Time) (marketingResearch, error) {
	var research marketingResearch
	if json.Unmarshal(raw, &research) != nil {
		return research, errors.New("invalid research")
	}
	if !marketingStrings(map[*string]int{&research.Takeaway: 1600, &research.HoldReason: 1000,
		&research.Rules.Links: 800, &research.Rules.SelfPromotion: 800, &research.Rules.AI: 800,
		&research.Rules.Flair: 800, &research.Rules.Cadence: 800}) {
		return research, errors.New("invalid research fields")
	}
	sources := []string{}
	seen := map[string]bool{}
	for _, source := range research.Sources {
		key := marketingSourceKey(source)
		if len(sources) >= 6 {
			break
		}
		if !seen[key] && evidence[key] && researchLinkMatches(target, source, true) {
			sources = append(sources, source)
			seen[key] = true
		}
	}
	examples := []marketingExample{}
	seen = map[string]bool{}
	for _, example := range research.Examples {
		if len(examples) >= 4 {
			break
		}
		key := marketingSourceKey(example.URL)
		if seen[key] || !evidence[key] || !researchLinkMatches(target, example.URL, false) || !marketingDate(example.Date) ||
			example.Date < checked.AddDate(0, -12, 0).Format(marketingDay) || example.Date > checked.Format(marketingDay) ||
			!marketingStrings(map[*string]int{&example.Title: 250, &example.Engagement: 300, &example.WhatWorked: 800}) || example.Title == "" {
			continue
		}
		seen[key] = true
		examples = append(examples, example)
	}
	research.Sources = sources
	research.Examples = examples
	if len(examples) == 0 {
		research.Takeaway = ""
	}
	if len(sources) == 0 {
		research.Hold = true
		research.HoldReason = "Current official rules for this destination could not be verified."
		research.Rules = marketingRules{}
	}
	for _, rule := range []string{research.Rules.Links, research.Rules.SelfPromotion, research.Rules.AI, research.Rules.Flair, research.Rules.Cadence} {
		text := strings.ToLower(rule)
		if text == "" || strings.Contains(text, "unclear") || strings.Contains(text, "not verified") || strings.Contains(text, "unknown") {
			research.Hold = true
		}
	}
	if research.Hold && research.HoldReason == "" {
		research.HoldReason = "Review the official rules and confirm permission before posting."
	}
	return research, nil
}

func updateMarketingResearch(c *gin.Context) {
	id, ok := marketingID(c, "target")
	if !ok {
		return
	}
	var in struct {
		Version uint `json:"version"`
	}
	if !marketingJSON(c, &in) {
		return
	}
	db := marketingDB(c)
	target, err := readTarget(db, id)
	if err != nil {
		marketingTargetError(c, err)
		return
	}
	if target.Version != in.Version {
		marketingError(c, 409, "This target changed. Reload before researching.")
		return
	}
	key, ok := startMarketingAI(c)
	if !ok {
		return
	}
	defer marketingGeneration.Unlock()
	checked := time.Now().UTC()
	raw, evidence, err := marketingAsk(c.Request.Context(), key, "cursemark_target_research", targetResearchInstructions,
		map[string]string{"destination": target.URL, "today": checked.Format(marketingDay)}, marketingResearchSchema(), true)
	if err != nil {
		marketingError(c, 502, "OpenAI did not return usable target research. The previous research was kept.")
		return
	}
	research, err := validateResearch(target.URL, raw, evidence, checked)
	if err != nil {
		marketingError(c, 502, "OpenAI returned invalid target research. The previous research was kept.")
		return
	}
	saved, _ := json.Marshal(research)
	update := db.Model(&models.MarketingTarget{}).Where("id = ? AND project = ? AND version = ?", id, marketingProject, in.Version).
		Updates(map[string]any{"research": string(saved), "research_url": target.URL, "checked_at": checked, "version": in.Version + 1})
	if update.Error != nil {
		marketingDBError(c, update.Error)
		return
	}
	if update.RowsAffected != 1 {
		marketingError(c, 409, "The target changed during research. No results were saved.")
		return
	}
	target.Version++
	target.CheckedAt = &checked
	target.ResearchURL = target.URL
	target.Research = string(saved)
	c.JSON(200, targetDetails(target))
}
