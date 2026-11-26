package endpoints

import (
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type marketingImpactObservation struct {
	Metric     string `json:"metric"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	ObservedOn string `json:"observed_on"`
	Value      *int64 `json:"value"`
	Comments   *int64 `json:"comments,omitempty"`
	Scope      string `json:"scope"`
	Note       string `json:"note,omitempty"`
}

type marketingImpactSummary struct {
	Level  int    `json:"level"`
	AsOf   string `json:"as_of,omitempty"`
	Metric string `json:"metric,omitempty"`
}

type marketingImpactDetail struct {
	Version uint                         `json:"version"`
	Items   []marketingImpactObservation `json:"items"`
}

type marketingTargetAttribution struct {
	MatchedRows uint64  `json:"matched_rows"`
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	Tracked     *uint64 `json:"tracked"`
	Wishlists   *uint64 `json:"wishlists"`
	Purchases   *uint64 `json:"purchases"`
	Activations *uint64 `json:"activations"`
}

var (
	impactVideoID    = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)
	impactRedditID   = regexp.MustCompile(`^[a-z0-9]{4,12}$`)
	errImpactVersion = errors.New("impact observations changed")
	errImpactSource  = errors.New("invalid impact observation")
)

func marketingPlatform(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Port() != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case domain(host, "reddit.com"):
		return "reddit"
	case domain(host, "youtube.com"), host == "youtu.be":
		return "youtube"
	case domain(host, "twitch.tv"):
		return "twitch"
	case domain(host, "x.com"), domain(host, "twitter.com"):
		return "x"
	default:
		return ""
	}
}

func impactVideo(raw string) string {
	if marketingPlatform(raw) != "youtube" {
		return ""
	}
	u, _ := url.Parse(raw)
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	id := ""
	switch {
	case strings.EqualFold(u.Hostname(), "youtu.be") && len(parts) == 1:
		id = parts[0]
	case u.Path == "/watch":
		id = u.Query().Get("v")
	case len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "live" || parts[0] == "embed"):
		id = parts[1]
	}
	if !impactVideoID.MatchString(id) {
		return ""
	}
	return id
}

func impactSubreddit(raw string) (string, []string) {
	if marketingPlatform(raw) != "reddit" {
		return "", nil
	}
	u, _ := url.Parse(raw)
	parts := strings.Split(strings.Trim(strings.ToLower(u.Path), "/"), "/")
	if len(parts) < 2 || parts[0] != "r" || !subredditName.MatchString(parts[1]) {
		return "", nil
	}
	return parts[1], parts[2:]
}

func impactSource(item marketingImpactObservation, target models.MarketingTarget) string {
	switch item.Metric {
	case "youtube_views", "youtube_short_views":
		if id := impactVideo(item.URL); target.IsCreator && id != "" {
			return "youtube:" + id
		}
	case "reddit_score":
		sub, parts := impactSubreddit(item.URL)
		targetSub, _ := impactSubreddit(target.URL)
		if !target.IsCreator && sub != "" && sub == targetSub &&
			len(parts) >= 2 && len(parts) <= 4 && parts[0] == "comments" && impactRedditID.MatchString(parts[1]) {
			key := "reddit:" + parts[1]
			if len(parts) == 4 {
				if !impactRedditID.MatchString(parts[3]) {
					return ""
				}
				key += ":" + parts[3]
			}
			return key
		}
	}
	return ""
}

func impactBand(item marketingImpactObservation) int {
	if item.Value == nil || item.Scope != "dedicated" {
		return 0
	}
	var ok, great int64
	switch item.Metric {
	case "youtube_views":
		ok, great = 1_000, 10_000
	case "youtube_short_views":
		ok, great = 3_000, 30_000
	case "reddit_score":
		ok, great = 10, 50
	default:
		return 0
	}
	switch {
	case *item.Value >= great:
		return 3
	case *item.Value >= ok:
		return 2
	default:
		return 1
	}
}

func summarizeImpact(detail marketingImpactDetail, target models.MarketingTarget) marketingImpactSummary {
	var score marketingImpactSummary
	for _, item := range detail.Items {
		level := impactBand(item)
		if level == 0 || impactSource(item, target) == "" {
			continue
		}
		if level > score.Level || (level == score.Level && item.ObservedOn > score.AsOf) {
			score = marketingImpactSummary{Level: level, AsOf: item.ObservedOn, Metric: item.Metric}
		}
	}
	return score
}

func decodeImpact(row models.MarketingTargetImpact) (marketingImpactDetail, error) {
	detail := marketingImpactDetail{Version: row.Version, Items: []marketingImpactObservation{}}
	if row.Observations == "" {
		return detail, nil
	}
	err := json.Unmarshal([]byte(row.Observations), &detail.Items)
	return detail, err
}

func readMarketingImpact(db *gorm.DB, ids []uint) (map[uint]marketingImpactDetail, error) {
	out := make(map[uint]marketingImpactDetail, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []models.MarketingTargetImpact
	if err := db.Where("target_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		detail, err := decodeImpact(row)
		if err != nil {
			return nil, err
		}
		out[row.TargetID] = detail
	}
	return out, nil
}

func targetSteamAttribution(db *gorm.DB, targetID uint) (*marketingTargetAttribution, error) {
	var stats marketingTargetAttribution
	err := db.Table("marketing_results AS v").
		Select(`COUNT(*) AS matched_rows, COALESCE(MIN(GREATEST(r.start_date, DATE_FORMAT(p.published_at, '%Y-%m-%d'))), '') AS start_date,
			COALESCE(MAX(r.end_date), '') AS end_date, SUM(v.tracked) AS tracked, SUM(v.wishlists) AS wishlists,
			SUM(v.purchases) AS purchases, SUM(v.activations) AS activations`).
		Joins("JOIN marketing_placements AS p ON p.id = v.placement_id").
		Joins("JOIN marketing_reports AS r ON r.id = v.report_id").
		Where("p.target_id = ? AND p.project = ? AND r.project = ? AND p.published_at IS NOT NULL AND r.end_date >= DATE(p.published_at)",
			targetID, marketingProject, marketingProject).
		Scan(&stats).Error
	if err != nil || stats.MatchedRows == 0 {
		return nil, err
	}
	return &stats, nil
}

func validateImpact(items []marketingImpactObservation, target models.MarketingTarget, coverage marketingCoverageFact) error {
	known := map[string]marketingCoverageItem{}
	if coverage.Status == "covered" {
		for _, entry := range coverage.Items {
			if id := impactVideo(entry.URL); id != "" {
				known[id] = entry
			}
		}
	}
	seen := map[string]bool{}
	for i := range items {
		item := &items[i]
		key := impactSource(*item, target)
		if key == "" || seen[key] || !safeResearchURL(item.URL) || !factDate(item.ObservedOn) ||
			!cleanEvidenceNote(&item.Note) || !marketingStrings(map[*string]int{&item.Title: 250}) ||
			item.Title == "" || contactPattern.MatchString(item.Title) || (item.Scope != "dedicated" && item.Scope != "shared") ||
			(item.Comments != nil && *item.Comments < 0) {
			return errImpactSource
		}
		seen[key] = true
		if item.Metric == "reddit_score" {
			continue
		}
		entry, exists := known[strings.TrimPrefix(key, "youtube:")]
		if !exists || (entry.Date != "" && item.ObservedOn < entry.Date) ||
			(entry.Format == "short") != (item.Metric == "youtube_short_views") ||
			(item.Value != nil && *item.Value < 0) {
			return errImpactSource
		}
		// Show the title from the independently checked coverage record.
		item.Title = entry.Title
	}
	return nil
}

func saveMarketingImpact(c *gin.Context) {
	id, ok := marketingID(c, "target")
	if !ok {
		return
	}
	var in struct {
		Version *uint                         `json:"version"`
		Items   *[]marketingImpactObservation `json:"observations"`
	}
	if !marketingJSON(c, &in) {
		return
	}
	if in.Version == nil || in.Items == nil || len(*in.Items) > 24 {
		marketingError(c, 400, "Provide the current impact version and up to 24 observations.")
		return
	}
	var detail marketingImpactDetail
	var target models.MarketingTarget
	err := marketingDB(c).Transaction(func(tx *gorm.DB) error {
		var err error
		target, err = readTarget(tx.Clauses(clause.Locking{Strength: "UPDATE"}), id)
		if err != nil {
			return err
		}
		var coverage marketingCoverageFact
		if target.IsCreator {
			evidence, err := creatorEvidenceView(tx, id)
			if err != nil {
				return err
			}
			coverage = evidence.Coverage
		}
		if err := validateImpact(*in.Items, target, coverage); err != nil {
			return err
		}
		var current models.MarketingTargetImpact
		err = tx.First(&current, id).Error
		creating := errors.Is(err, gorm.ErrRecordNotFound)
		if creating {
			current.TargetID = id
		} else if err != nil {
			return err
		}
		if current.Version != *in.Version {
			return errImpactVersion
		}
		raw, err := json.Marshal(*in.Items)
		if err != nil {
			return err
		}
		current.Observations = string(raw)
		current.Version++
		detail = marketingImpactDetail{Version: current.Version, Items: *in.Items}
		if creating {
			return tx.Create(&current).Error
		}
		return tx.Save(&current).Error
	})
	switch {
	case errors.Is(err, errImpactVersion):
		marketingError(c, 409, "Impact observations changed. Read the current target before saving.")
	case errors.Is(err, errImpactSource):
		marketingError(c, 400, "Use distinct verified Cursemark coverage URLs or Casey's posts from this subreddit. Check the metric, snapshot date and scope.")
	case err != nil:
		marketingTargetError(c, err)
	default:
		c.JSON(200, gin.H{"impact": summarizeImpact(detail, target), "impact_evidence": detail})
	}
}
