package endpoints

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxMarketingCSV = 4 << 20

type marketingCSVRow struct {
	content, campaign string
	counts            models.MarketingCounts
}

var marketingColumns = map[string]string{
	"utmsource": "source", "source": "source", "utmmedium": "medium", "medium": "medium",
	"utmcampaign": "campaign", "campaign": "campaign", "utmcontent": "content", "content": "content",
	"utmterm": "term", "term": "term", "appid": "app",
	"totalvisits": "total", "trustedvisits": "trusted", "trackedvisits": "tracked",
	"wishlists": "wishlists", "wishlist": "wishlists", "wishlistadditions": "wishlists",
	"purchase": "purchases", "purchases": "purchases", "activation": "activations", "activations": "activations",
}

func marketingCSVHeader(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, strings.ToLower(s))
}

func marketingCSVCount(s string) (*uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "—" || strings.EqualFold(s, "N/A") {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	for i, part := range parts {
		if part == "" || (i == 0 && len(parts) > 1 && len(part) > 3) || (i > 0 && len(part) != 3) {
			return nil, errors.New("invalid count")
		}
	}
	n, err := strconv.ParseUint(strings.Join(parts, ""), 10, 32)
	return &n, err
}

func readMarketingCSV(data []byte) ([]marketingCSVRow, error) {
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))))
	head, err := reader.Read()
	if err != nil {
		return nil, errors.New("Choose a Steamworks UTM breakdown CSV.")
	}
	index := map[string]int{}
	for i, value := range head {
		key := marketingColumns[marketingCSVHeader(value)]
		if key == "" {
			continue
		}
		if _, exists := index[key]; exists {
			return nil, errors.New("The CSV has duplicate Steamworks columns.")
		}
		index[key] = i
	}
	for _, key := range []string{"source", "medium", "campaign", "content", "total", "tracked"} {
		if _, exists := index[key]; !exists {
			return nil, errors.New("Use the English UTM breakdown export with all UTM fields, Total Visits and Tracked Visits.")
		}
	}
	seen := map[string]bool{}
	rows := []marketingCSVRow{}
	for line := 2; ; line++ {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || line > 30001 {
			return nil, fmt.Errorf("Invalid or oversized Steamworks CSV near row %d.", line)
		}
		cell := func(column string) string {
			if i, ok := index[column]; ok {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		if app := cell("app"); app != "" && app != "3219180" {
			return nil, errors.New("This CSV includes an App ID other than Cursemark (3219180).")
		}
		content, campaign := cell("content"), cell("campaign")
		if cell("source") != "clydegames" || cell("medium") != "community" || cell("term") != "" ||
			!strings.HasPrefix(campaign, "cursemark-beat-") || len(content) != 32 || strings.ToLower(content) != content {
			continue
		}
		if _, err := hex.DecodeString(content); err != nil {
			continue
		}
		if seen[content] {
			return nil, fmt.Errorf("Duplicate placement on row %d. Use a single breakdown export.", line)
		}
		seen[content] = true
		var counts models.MarketingCounts
		fields := []struct {
			name string
			dest **uint64
		}{
			{"total", &counts.Total}, {"trusted", &counts.Trusted}, {"tracked", &counts.Tracked},
			{"wishlists", &counts.Wishlists}, {"purchases", &counts.Purchases}, {"activations", &counts.Activations},
		}
		for _, f := range fields {
			n, err := marketingCSVCount(cell(f.name))
			if err != nil {
				return nil, fmt.Errorf("Invalid Steamworks count on row %d.", line)
			}
			*f.dest = n
		}
		rows = append(rows, marketingCSVRow{content: content, campaign: campaign, counts: counts})
	}
	return rows, nil
}

func listMarketingReports(c *gin.Context) {
	var reports []models.MarketingReport
	if err := marketingDB(c).Where("project = ?", marketingProject).Order("start_date DESC").Find(&reports).Error; err != nil {
		marketingDBError(c, err)
		return
	}
	c.JSON(200, gin.H{"reports": reports})
}

var errMarketingOverlap = errors.New("overlapping Steamworks report")

func saveMarketingReport(db *gorm.DB, start, end, fileHash string, rows []marketingCSVRow) (matched int, duplicate bool, err error) {
	err = db.Transaction(func(tx *gorm.DB) error {
		// Lock the stable game registry row, including for the first import, so
		// concurrent imports cannot both miss an overlapping window.
		var project models.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").
			Where("id = ?", marketingProject).Take(&project).Error; err != nil {
			return err
		}
		var report models.MarketingReport
		err := tx.Where("project = ? AND start_date <= ? AND end_date >= ?", marketingProject, end, start).Take(&report).Error
		switch {
		case err == nil && (report.StartDate != start || report.EndDate != end):
			return errMarketingOverlap
		case errors.Is(err, gorm.ErrRecordNotFound):
			report = models.MarketingReport{Project: marketingProject, StartDate: start, EndDate: end, ImportedAt: time.Now().UTC()}
			if err := tx.Create(&report).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		}
		var count int64
		if err := tx.Model(&models.MarketingImport{}).Where("report_id = ? AND file_sha256 = ?", report.ID, fileHash).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			duplicate = true
			return nil
		}
		var posts []models.MarketingPlacement
		if err := tx.Select("id", "beat_id", "utm").Where("project = ?", marketingProject).Find(&posts).Error; err != nil {
			return err
		}
		postByToken := make(map[string]models.MarketingPlacement, len(posts))
		for _, post := range posts {
			postByToken[post.UTM] = post
		}
		var results []models.MarketingResult
		for _, row := range rows {
			if post, ok := postByToken[row.content]; ok && row.campaign == marketingCampaign(post.BeatID) {
				results = append(results, models.MarketingResult{ReportID: report.ID, PlacementID: post.ID, MarketingCounts: row.counts})
			}
		}
		if err := tx.Where("report_id = ?", report.ID).Delete(&models.MarketingResult{}).Error; err != nil {
			return err
		}
		if len(results) != 0 {
			if err := tx.CreateInBatches(results, 250).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&report).Update("imported_at", time.Now().UTC()).Error; err != nil {
			return err
		}
		matched = len(results)
		return tx.Create(&models.MarketingImport{ReportID: report.ID, FileSHA256: fileHash}).Error
	})
	return
}

func importMarketingReport(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMarketingCSV+(16<<10))
	if err := c.Request.ParseMultipartForm(maxMarketingCSV + (16 << 10)); err != nil {
		marketingError(c, 400, "Upload a Steamworks CSV up to 4 MB.")
		return
	}
	defer c.Request.MultipartForm.RemoveAll()
	start, end := c.PostForm("start_date"), c.PostForm("end_date")
	if !marketingDate(start) || !marketingDate(end) || start > end || end > time.Now().UTC().Format(marketingDay) ||
		c.PostForm("full_breakdown") != "yes" {
		marketingError(c, 400, "Confirm a full Cursemark export and enter its inclusive date range.")
		return
	}
	file, _, err := c.Request.FormFile("csv")
	if err != nil {
		marketingError(c, 400, "Choose a Steamworks UTM breakdown CSV.")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxMarketingCSV+1))
	if err != nil || len(data) > maxMarketingCSV {
		marketingError(c, 400, "Upload a Steamworks CSV up to 4 MB.")
		return
	}
	rows, err := readMarketingCSV(data)
	if err != nil {
		marketingError(c, 400, err.Error())
		return
	}
	hash := sha256.Sum256(data)
	matched, duplicate, err := saveMarketingReport(marketingDB(c), start, end, hex.EncodeToString(hash[:]), rows)
	if errors.Is(err, errMarketingOverlap) {
		marketingError(c, 409, "This range overlaps an import. Export the exact same range to refresh it, or choose dates outside all imported ranges.")
		return
	}
	if err != nil {
		marketingDBError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"matched": matched, "duplicate": duplicate})
}

type marketingSummary struct {
	models.MarketingCounts
	Expected int `json:"expected"`
	Observed int `json:"observed"`
}

func addMarketingCount(dst **uint64, n *uint64) {
	if n != nil {
		if *dst == nil {
			*dst = new(uint64)
		}
		**dst += *n
	}
}

func (s *marketingSummary) add(n models.MarketingCounts) {
	addMarketingCount(&s.Total, n.Total)
	addMarketingCount(&s.Trusted, n.Trusted)
	addMarketingCount(&s.Tracked, n.Tracked)
	addMarketingCount(&s.Wishlists, n.Wishlists)
	addMarketingCount(&s.Purchases, n.Purchases)
	addMarketingCount(&s.Activations, n.Activations)
}

func marketingResults(db *gorm.DB, beatID uint) ([]marketingPost, marketingSummary, error) {
	var posts []models.MarketingPlacement
	var reports []models.MarketingReport
	var results []models.MarketingResult
	aggregate := marketingSummary{}
	// Keep report metadata and result rows in the same consistent snapshot
	// when an import is refreshing an existing range.
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project = ? AND beat_id = ?", marketingProject, beatID).
			Order("rank = 0, rank ASC, id ASC").Find(&posts).Error; err != nil {
			return err
		}
		if len(posts) == 0 {
			return nil
		}
		if err := tx.Where("project = ?", marketingProject).Find(&reports).Error; err != nil {
			return err
		}
		query := tx.Model(&models.MarketingPlacement{}).Select("id").Where("project = ? AND beat_id = ?", marketingProject, beatID)
		return tx.Where("placement_id IN (?)", query).Find(&results).Error
	})
	if err != nil {
		return nil, aggregate, err
	}
	resultByID := make(map[[2]uint]models.MarketingCounts, len(results))
	for _, r := range results {
		resultByID[[2]uint{r.PlacementID, r.ReportID}] = r.MarketingCounts
	}
	out := make([]marketingPost, 0, len(posts))
	for _, post := range posts {
		summary := marketingSummary{}
		if post.PublishedAt != nil {
			day := post.PublishedAt.UTC().Format(marketingDay)
			for _, report := range reports {
				if report.EndDate < day {
					continue
				}
				summary.Expected++
				counts, ok := resultByID[[2]uint{post.ID, report.ID}]
				if ok {
					summary.Observed++
					summary.add(counts)
				}
			}
			aggregate.add(summary.MarketingCounts)
			aggregate.Expected += summary.Expected
			aggregate.Observed += summary.Observed
		}
		out = append(out, marketingPost{MarketingPlacement: post, SteamURL: marketingUTM(post), Result: summary})
	}
	return out, aggregate, nil
}
