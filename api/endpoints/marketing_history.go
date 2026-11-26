package endpoints

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type marketingHistoryFields struct {
	OutreachStatus string   `json:"outreach_status"`
	ActivityStatus string   `json:"activity_status"`
	CheckedOn      string   `json:"checked_on"`
	Sources        []string `json:"sources"`
	Note           string   `json:"note,omitempty"`
}

type marketingHistoryView struct {
	Version uint `json:"version"`
	marketingHistoryFields
}

type marketingNewStatus struct {
	IsNew bool   `json:"is_new"`
	Label string `json:"label"`
}

func marketingHistorySummary(target models.MarketingTarget, history marketingHistoryView, coverage string) *marketingNewStatus {
	if history.Version == 0 {
		return nil
	}
	if target.IsCreator {
		switch {
		case history.OutreachStatus == "none_found":
			return &marketingNewStatus{IsNew: true, Label: "New contact"}
		case coverage == "no_result":
			return &marketingNewStatus{IsNew: true, Label: "No prior coverage found"}
		}
	} else if history.ActivityStatus == "none_found" {
		return &marketingNewStatus{IsNew: true, Label: "No prior public activity found"}
	} else if history.OutreachStatus == "none_found" {
		return &marketingNewStatus{IsNew: true, Label: "New contact"}
	}
	return nil
}

func readMarketingHistory(db *gorm.DB, ids []uint) (map[uint]marketingHistoryView, error) {
	out := make(map[uint]marketingHistoryView, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []models.MarketingTargetHistory
	if err := db.Where("target_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		view := marketingHistoryView{Version: row.Version}
		if err := json.Unmarshal([]byte(row.History), &view.marketingHistoryFields); err != nil {
			return nil, err
		}
		out[row.TargetID] = view
	}
	return out, nil
}

func marketingCoveredStatuses(db *gorm.DB, ids []uint) (map[uint]string, error) {
	out := make(map[uint]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []models.MarketingCreatorEvidence
	if err := db.Select("target_id", "coverage").Where("target_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Coverage == "" {
			continue
		}
		var coverage marketingCoverageFact
		if err := json.Unmarshal([]byte(row.Coverage), &coverage); err != nil {
			return nil, err
		}
		out[row.TargetID] = coverage.Status
	}
	return out, nil
}

func validateMarketingHistory(target models.MarketingTarget, fields *marketingHistoryFields) bool {
	if !factDate(fields.CheckedOn) || !cleanEvidenceNote(&fields.Note) || len(fields.Sources) > 6 {
		return false
	}
	switch fields.OutreachStatus {
	case "contacted", "inbound_first", "none_found", "identity_unverified", "not_checked", "not_applicable":
	default:
		return false
	}
	switch fields.ActivityStatus {
	case "post_found", "coverage_found", "none_found", "not_checked":
	default:
		return false
	}
	community, _ := impactSubreddit(target.URL)
	if target.IsCreator && (fields.ActivityStatus != "not_checked" || fields.OutreachStatus == "not_applicable") {
		return false
	}
	if !target.IsCreator && community != "" && (fields.OutreachStatus != "not_applicable" || fields.ActivityStatus == "coverage_found") {
		return false
	}
	if (fields.ActivityStatus == "post_found" || fields.ActivityStatus == "coverage_found") && len(fields.Sources) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, source := range fields.Sources {
		if !safeResearchURL(source) {
			return false
		}
		if !target.IsCreator {
			u, _ := url.Parse(source)
			t, _ := url.Parse(target.URL)
			if community != "" {
				sub, parts := impactSubreddit(source)
				if sub != community || (fields.ActivityStatus == "post_found" && (len(parts) < 2 || parts[0] != "comments")) {
					return false
				}
			} else if !strings.EqualFold(strings.TrimPrefix(strings.ToLower(u.Hostname()), "www."), strings.TrimPrefix(strings.ToLower(t.Hostname()), "www.")) {
				return false
			}
		}
		key := marketingSourceKey(source)
		if seen[key] {
			return false
		}
		seen[key] = true
	}
	if fields.Sources == nil {
		fields.Sources = []string{}
	}
	return true
}

var errMarketingHistoryInvalid = errors.New("invalid target history")
var errMarketingHistoryVersion = errors.New("target history changed")

func saveMarketingHistory(c *gin.Context) {
	id, ok := marketingID(c, "target")
	if !ok {
		return
	}
	var input struct {
		Version *uint                   `json:"version"`
		History *marketingHistoryFields `json:"history"`
	}
	if !marketingJSON(c, &input) {
		return
	}
	if input.Version == nil || input.History == nil {
		marketingError(c, 400, "Provide the current history version and checked statuses.")
		return
	}
	view := marketingHistoryView{marketingHistoryFields: *input.History}
	err := marketingDB(c).Transaction(func(tx *gorm.DB) error {
		target, err := readTarget(tx.Clauses(clause.Locking{Strength: "UPDATE"}), id)
		if err != nil {
			return err
		}
		if !validateMarketingHistory(target, &view.marketingHistoryFields) {
			return errMarketingHistoryInvalid
		}
		var row models.MarketingTargetHistory
		err = tx.First(&row, id).Error
		creating := errors.Is(err, gorm.ErrRecordNotFound)
		if creating {
			row.TargetID = id
		} else if err != nil {
			return err
		}
		if row.Version != *input.Version {
			return errMarketingHistoryVersion
		}
		raw, err := json.Marshal(view.marketingHistoryFields)
		if err != nil {
			return err
		}
		row.History = string(raw)
		row.Version++
		view.Version = row.Version
		if creating {
			return tx.Create(&row).Error
		}
		return tx.Save(&row).Error
	})
	switch {
	case errors.Is(err, errMarketingHistoryInvalid):
		marketingError(c, 400, "Check the status, snapshot date and exact public sources. Keep addresses and private correspondence out.")
	case errors.Is(err, errMarketingHistoryVersion):
		marketingError(c, 409, "This target history changed. Read the target before saving.")
	case err != nil:
		marketingTargetError(c, err)
	default:
		c.JSON(200, view)
	}
}
