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
	Rules         marketingRules     `json:"rules"`
	Sources       []string           `json:"sources"`
	Examples      []marketingExample `json:"examples"`
	Takeaway      string             `json:"takeaway"`
	Hold          bool               `json:"hold"`
	HoldReason    string             `json:"hold_reason"`
	AIRestriction string             `json:"ai_restriction,omitempty"`
	Games         []string           `json:"games_covered,omitempty"`
}

type marketingTargetView struct {
	ID              uint                    `json:"id"`
	Version         uint                    `json:"version"`
	Name            string                  `json:"name"`
	URL             string                  `json:"url"`
	ResearchURL     string                  `json:"research_url"`
	CheckedAt       *time.Time              `json:"checked_at"`
	Research        *marketingResearch      `json:"research"`
	Kind            string                  `json:"kind"`
	ResearchReady   bool                    `json:"research_ready"`
	ResearchHold    bool                    `json:"research_hold"`
	CreatorPlatform string                  `json:"creator_platform,omitempty"`
	Creator         *marketingCreatorFields `json:"creator,omitempty"`
}

var subredditName = regexp.MustCompile("^[a-zA-Z0-9_]{2,30}$")

func targetURL(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !safeResearchURL(value) {
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
	key := host + strings.TrimRight(u.Path, "/")
	if domain(host, "youtube.com") && u.Path == "/watch" {
		key += "?v=" + u.Query().Get("v")
	}
	return key
}

func targetDetails(target models.MarketingTarget) marketingTargetView {
	out := marketingTargetView{ID: target.ID, Version: target.Version, Name: target.Name, URL: target.URL,
		ResearchURL: target.ResearchURL, CheckedAt: target.CheckedAt, Kind: "destination"}
	if target.IsCreator {
		out.Kind = "creator"
		out.ResearchURL = ""
	}
	if target.Research != "" {
		var research marketingResearch
		if json.Unmarshal([]byte(target.Research), &research) == nil {
			out.Research = &research
			out.ResearchHold = research.Hold
			out.ResearchReady = target.CheckedAt != nil && target.ResearchURL == target.URL
		}
	}
	return out
}

func targetPrivateDetails(db *gorm.DB, target models.MarketingTarget) (marketingTargetView, error) {
	out := targetDetails(target)
	if target.IsCreator {
		person, err := readMarketingCreator(db, target.ID)
		if err != nil {
			return out, err
		}
		fields := creatorFields(person)
		out.Creator = &fields
		out.CreatorPlatform = fields.Platform
		out.ResearchReady = out.Research != nil && target.CheckedAt != nil && target.ResearchURL == creatorResearchKey(person)
	}
	return out, nil
}

func getMarketingTarget(c *gin.Context) {
	id, ok := marketingID(c, "target")
	if !ok {
		return
	}
	db := marketingDB(c)
	target, err := readTarget(db, id)
	if err != nil {
		marketingTargetError(c, err)
		return
	}
	out, err := targetPrivateDetails(db, target)
	if err != nil {
		marketingDBError(c, err)
		return
	}
	c.JSON(200, out)
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
	creatorKeys := map[uint]string{}
	platforms := map[uint]string{}
	var creators []models.MarketingCreator
	query := marketingDB(c).Model(&models.MarketingTarget{}).Select("id").Where("project = ? AND is_creator = ?", marketingProject, true)
	if err := marketingDB(c).Select("target_id", "platform", "channel_urls").Where("target_id IN (?)", query).Find(&creators).Error; err != nil {
		marketingDBError(c, err)
		return
	}
	for _, person := range creators {
		creatorKeys[person.TargetID] = creatorResearchKey(person)
		platforms[person.TargetID] = person.Platform
	}
	for _, target := range targets {
		summary := targetDetails(target)
		if target.IsCreator {
			summary.ResearchReady = summary.Research != nil && target.CheckedAt != nil && target.ResearchURL == creatorKeys[target.ID]
			summary.CreatorPlatform = platforms[target.ID]
		}
		summary.Research = nil
		out = append(out, summary)
	}
	c.JSON(200, gin.H{"targets": out})
}

func saveMarketingTarget(c *gin.Context) {
	var in marketingTargetInput
	if !marketingJSON(c, &in) {
		return
	}
	if in.Kind == "creator" {
		saveMarketingCreator(c, in)
		return
	}
	if in.Kind != "" && in.Kind != "destination" {
		marketingError(c, 400, "Choose a public destination or creator contact.")
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
	update := db.Model(&models.MarketingTarget{}).Where("id = ? AND project = ? AND version = ? AND is_creator = ?", id, marketingProject, in.Version, false).
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

const targetResearchInstructions = `Research only the exact public destination URL for Cursemark, a dark fantasy action roguelite. Find up to four actual game-promotion posts from the past twelve months, favoring six months, with exact permalinks, YYYY-MM-DD dates, and visible engagement. Leave engagement empty when unavailable. Distinguish observed patterns from inference; past posts are not permission. Find current official rules for links, self-promotion, flair, cadence, approval and AI restrictions. Use Unclear when a rule cannot be confirmed. Cite official sources separately. An editor personally rewrites and finalizes the private first draft. Unknown AI rules or a private first draft alone are not a Hold. Set hold=true only for genuine publishing restrictions, unresolved publishing permission, approval requirements, or conflicting guidance. In ai_restriction use assisted_submissions for explicit coverage of AI-assisted submissions or drafting, final_content for a stated rule applying to the public content, or unspecified when not confirmed. Describe applicable AI rules accurately. Do not collect contact details. Return games_covered as an empty array. Use plain text.`

func marketingResearchSchema() map[string]any {
	props := marketingTextFields("takeaway", "hold_reason", "ai_restriction")
	props["hold"] = map[string]string{"type": "boolean"}
	props["sources"] = map[string]any{"type": "array", "items": map[string]string{"type": "string"}}
	props["games_covered"] = map[string]any{"type": "array", "items": map[string]string{"type": "string"}}
	props["rules"] = marketingObject(marketingTextFields("links", "self_promotion", "ai", "flair", "cadence"))
	props["examples"] = map[string]any{"type": "array", "items": marketingObject(marketingTextFields("title", "url", "date", "engagement", "what_worked"))}
	return marketingObject(props)
}

func researchLinkMatches(target, source string, official bool) bool {
	if !safeResearchURL(source) {
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

func validateResearch(target string, raw []byte, evidence map[string]bool, checked time.Time, channelURLs ...string) (marketingResearch, error) {
	var research marketingResearch
	if json.Unmarshal(raw, &research) != nil {
		return research, errors.New("invalid research")
	}
	cleanResearchText(&research)
	isCreator := len(channelURLs) > 0
	matches := func(source string, official bool) bool {
		if isCreator {
			return creatorEvidence(channelURLs, source, official)
		}
		return researchLinkMatches(target, source, official)
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
		if !seen[key] && evidence[key] && matches(source, true) {
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
		if seen[key] || !evidence[key] || !matches(example.URL, false) || !marketingDate(example.Date) ||
			example.Date < checked.AddDate(0, -12, 0).Format(marketingDay) || example.Date > checked.Format(marketingDay) ||
			!marketingStrings(map[*string]int{&example.Title: 250, &example.Engagement: 300, &example.WhatWorked: 800}) || example.Title == "" {
			continue
		}
		seen[key] = true
		examples = append(examples, example)
	}
	research.Sources = sources
	research.Examples = examples
	if isCreator {
		research.Rules = marketingRules{}
		research.AIRestriction = ""
		if len(sources) == 0 && len(examples) == 0 {
			research.Hold = true
			research.HoldReason = "No recent creator coverage could be verified from the public channels."
			research.Games = nil
		}
		return research, nil
	}
	research.Games = nil
	if len(examples) == 0 {
		research.Takeaway = ""
	}
	if len(sources) == 0 {
		research.Hold = true
		research.HoldReason = "Current official rules for this destination could not be verified."
		research.Rules = marketingRules{}
		research.AIRestriction = "unspecified"
	}
	if research.AIRestriction == "assisted_submissions" {
		research.Hold = true
		if research.HoldReason == "" {
			research.HoldReason = "The official rules expressly cover AI-assisted submissions. Review the restriction before posting."
		}
	} else if research.AIRestriction != "final_content" {
		research.AIRestriction = "unspecified"
	}
	for _, rule := range []string{research.Rules.Links, research.Rules.SelfPromotion, research.Rules.Flair, research.Rules.Cadence} {
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
	researchURL := target.URL
	instructions := targetResearchInstructions
	input := map[string]any{"destination": target.URL}
	var links []string
	if !target.IsCreator && !safeResearchURL(target.URL) {
		marketingError(c, 400, "Save a public destination URL before researching.")
		return
	}
	if target.IsCreator {
		person, err := readMarketingCreator(db, target.ID)
		if err != nil {
			marketingDBError(c, err)
			return
		}
		fields := creatorFields(person)
		for _, link := range fields.ChannelURLs {
			if canonical, ok := creatorChannel(link); ok {
				links = append(links, canonical)
			}
		}
		if len(links) == 0 {
			marketingError(c, 400, "Add a verified public channel URL before researching this creator.")
			return
		}
		// Keep the public provider request an allowlist. No contact fields or
		// correspondence enter either the model context or web search.
		input = map[string]any{"platform": fields.Platform, "public_channel_urls": links}
		researchURL = creatorResearchKey(person)
		instructions = creatorResearchInstructions
	}
	key, ok := startMarketingAI(c)
	if !ok {
		return
	}
	defer marketingGeneration.Unlock()
	checked := time.Now().UTC()
	input["today"] = checked.Format(marketingDay)
	raw, evidence, err := marketingAsk(c.Request.Context(), key, "cursemark_target_research", instructions, input, marketingResearchSchema(), true)
	if err != nil {
		marketingError(c, 502, "OpenAI did not return usable target research. The previous research was kept.")
		return
	}
	research, err := validateResearch(target.URL, raw, evidence, checked, links...)
	if err != nil {
		marketingError(c, 502, "OpenAI returned invalid target research. The previous research was kept.")
		return
	}
	saved, _ := json.Marshal(research)
	update := db.Model(&models.MarketingTarget{}).Where("id = ? AND project = ? AND version = ?", id, marketingProject, in.Version).
		Updates(map[string]any{"research": string(saved), "research_url": researchURL, "checked_at": checked, "version": in.Version + 1})
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
	target.ResearchURL = researchURL
	target.Research = string(saved)
	out, err := targetPrivateDetails(db, target)
	if err != nil {
		marketingDBError(c, err)
		return
	}
	c.JSON(200, out)
}
