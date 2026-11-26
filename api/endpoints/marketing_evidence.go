package endpoints

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type marketingCoverageItem struct {
	Title  string `json:"title"`
	URL    string `json:"url"`
	Date   string `json:"date"`
	Format string `json:"format"`
}

type marketingCoverageFact struct {
	Status    string                  `json:"status"`
	CheckedAt string                  `json:"checked_at"`
	Note      string                  `json:"note"`
	Items     []marketingCoverageItem `json:"items"`
}

type marketingReplyFact struct {
	Status    string `json:"status"`
	CheckedAt string `json:"checked_at"`
	Date      string `json:"date"`
	Note      string `json:"note"`
}

type marketingCreatorEvidenceView struct {
	Version  uint                  `json:"version"`
	Coverage marketingCoverageFact `json:"coverage"`
	Reply    marketingReplyFact    `json:"reply"`
	AppID    string                `json:"steam_app_id"`
}

func cleanEvidenceNote(note *string) bool {
	if !marketingStrings(map[*string]int{note: 1000}) || contactPattern.MatchString(*note) {
		return false
	}
	text := strings.ToLower(*note)
	return !strings.Contains(text, "https://") && !strings.Contains(text, "http://") && !strings.Contains(text, "mailto:")
}

func factDate(date string) bool {
	return marketingDate(date) && date <= time.Now().UTC().Format(marketingDay)
}

func validateCoverage(v *marketingCoverageFact) bool {
	if !cleanEvidenceNote(&v.Note) || len(v.Items) > 8 {
		return false
	}
	switch v.Status {
	case "not_checked":
		return v.CheckedAt == "" && len(v.Items) == 0
	case "covered":
		if len(v.Items) == 0 {
			return false
		}
	case "no_result", "inaccessible", "self_reported":
		if len(v.Items) > 0 {
			return false
		}
	default:
		return false
	}
	if !factDate(v.CheckedAt) {
		return false
	}
	urls := map[string]bool{}
	for i := range v.Items {
		item := &v.Items[i]
		if !marketingStrings(map[*string]int{&item.Title: 250, &item.URL: 1000}) || item.Title == "" ||
			!safeResearchURL(item.URL) || urls[item.URL] || (item.Date != "" && (!factDate(item.Date) || item.Date > v.CheckedAt)) ||
			contactPattern.MatchString(item.Title) {
			return false
		}
		switch item.Format {
		case "video", "short", "stream", "post", "article", "other":
		default:
			return false
		}
		urls[item.URL] = true
	}
	if v.Items == nil {
		v.Items = []marketingCoverageItem{}
	}
	return true
}

func validateReply(v *marketingReplyFact) bool {
	if !cleanEvidenceNote(&v.Note) {
		return false
	}
	switch v.Status {
	case "not_checked":
		return v.CheckedAt == "" && v.Date == ""
	case "no_reply_found":
		return factDate(v.CheckedAt) && v.Date == ""
	case "replied_to_outreach", "inbound_first", "bounced":
		return factDate(v.CheckedAt) && factDate(v.Date) && v.Date <= v.CheckedAt
	}
	return false
}

func creatorEvidenceView(db *gorm.DB, id uint) (marketingCreatorEvidenceView, error) {
	var stored models.MarketingCreatorEvidence
	out := marketingCreatorEvidenceView{AppID: "3219180", Coverage: marketingCoverageFact{Status: "not_checked", Items: []marketingCoverageItem{}},
		Reply: marketingReplyFact{Status: "not_checked"}}
	err := db.First(&stored, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.Version = stored.Version
	if stored.Coverage != "" {
		if err := json.Unmarshal([]byte(stored.Coverage), &out.Coverage); err != nil {
			return out, err
		}
	}
	if stored.Reply != "" {
		if err := json.Unmarshal([]byte(stored.Reply), &out.Reply); err != nil {
			return out, err
		}
	}
	return out, nil
}

var errCreatorEvidenceVersion = errors.New("creator report changed")

func saveCreatorEvidence(c *gin.Context) {
	id, ok := marketingID(c, "target")
	if !ok {
		return
	}
	var in struct {
		Version  uint                   `json:"version"`
		Coverage *marketingCoverageFact `json:"coverage"`
		Reply    *marketingReplyFact    `json:"reply"`
	}
	if !marketingJSON(c, &in) {
		return
	}
	if (in.Coverage == nil && in.Reply == nil) || (in.Coverage != nil && !validateCoverage(in.Coverage)) ||
		(in.Reply != nil && !validateReply(in.Reply)) {
		marketingError(c, 400, "Provide valid coverage or reply facts, dated public sources where verified, and a summary without addresses or private links.")
		return
	}
	db := marketingDB(c)
	err := db.Transaction(func(tx *gorm.DB) error {
		target, err := readTarget(tx.Clauses(clause.Locking{Strength: "UPDATE"}), id)
		if err != nil {
			return err
		}
		if !target.IsCreator {
			return gorm.ErrRecordNotFound
		}
		var row models.MarketingCreatorEvidence
		err = tx.First(&row, id).Error
		creating := errors.Is(err, gorm.ErrRecordNotFound)
		if creating {
			row.TargetID = id
		} else if err != nil {
			return err
		}
		if row.Version != in.Version {
			return errCreatorEvidenceVersion
		}
		if in.Coverage != nil {
			data, _ := json.Marshal(in.Coverage)
			row.Coverage = string(data)
		}
		if in.Reply != nil {
			data, _ := json.Marshal(in.Reply)
			row.Reply = string(data)
		}
		row.Version++
		if creating {
			return tx.Create(&row).Error
		}
		return tx.Save(&row).Error
	})
	if errors.Is(err, errCreatorEvidenceVersion) {
		marketingError(c, 409, "The creator report changed. Read its current version before updating.")
		return
	}
	if err != nil {
		marketingTargetError(c, err)
		return
	}
	evidence, err := creatorEvidenceView(db, id)
	if err != nil {
		marketingDBError(c, err)
		return
	}
	c.JSON(200, evidence)
}
