package models

import (
	"database/sql"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Canonical group log, with original per-report notes retained as read-only history.
type CrashInvestigationNote struct {
	Scope          string    `json:"scope"`
	ID             uint      `json:"id"`
	ReportID       uint      `json:"report_id"`
	CreatedAt      time.Time `json:"created_at"`
	Actor          string    `json:"actor"`
	Note           string    `json:"note"`
	State          string    `json:"state,omitempty"`
	ReleaseState   string    `json:"release_state,omitempty"`
	ReportResolved bool      `json:"report_resolved"`
	ReportBuild    string    `json:"report_build,omitempty"`
	ReportGroupKey string    `json:"report_group_key,omitempty"`
}

type CrashInvestigationLog struct {
	ReportIDs               []uint                   `json:"report_ids"`
	ReportIndexComplete     bool                     `json:"report_index_complete"`
	Investigation           *CrashWorkView           `json:"investigation"`
	Project                 string                   `json:"project"`
	AnchorID                uint                     `json:"anchor_id"`
	GroupKey                string                   `json:"group_key,omitempty"`
	Family                  string                   `json:"family,omitempty"`
	ReportCount             int64                    `json:"report_count"`
	OpenReportCount         int64                    `json:"open_report_count"`
	NoteCount               int64                    `json:"note_count"`
	Items                   []CrashInvestigationNote `json:"items"`
	HasMore                 bool                     `json:"has_more"`
	NextAfterID             uint64                   `json:"next_after_id"`
	FrontierID              uint64                   `json:"frontier_id"`
	Limit                   int                      `json:"limit"`
	IncludesResolvedReports bool                     `json:"includes_resolved_reports"`
}

func ReadCrashInvestigationLog(db *gorm.DB, id uint, project, family string, after uint64, limit int) (CrashInvestigationLog, error) {
	if id == 0 || project != NormalizeProjectID(project) || len(project) > 64 || limit < 1 || limit > 50 || (family != "" && family != "access_violations") {
		return CrashInvestigationLog{}, ErrNoteInput
	}
	var out CrashInvestigationLog
	e := db.Transaction(func(tx *gorm.DB) error {
		var err error
		out, err = readCrashInvestigationSnapshot(tx, id, project, family, after, limit)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return out, e
}

func readCrashInvestigationSnapshot(db *gorm.DB, id uint, project, family string, after uint64, limit int) (CrashInvestigationLog, error) {
	out := CrashInvestigationLog{Project: project, AnchorID: id, Items: []CrashInvestigationNote{}, NextAfterID: after, Limit: limit, IncludesResolvedReports: true}
	if limit < 1 || limit > 50 || (family != "" && family != "access_violations") {
		return out, ErrNoteInput
	}
	scope, e := ResolveCrashWorkScope(db, id, project)
	if e != nil {
		return out, e
	}
	if family != "" && scope.Family != family {
		return out, ErrNoteInput
	}
	unit := CrashInvestigationUnit{}
	e = db.Where("`key`=?", scope.Key).Take(&unit).Error
	if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
		return out, e
	}
	view, e := scope.View(db, unit)
	if e != nil {
		return out, e
	}
	out.ReportIDs = []uint{}
	if e = scope.Reports(db).Select("c.id").Order("c.id ASC").Limit(100001).Pluck("c.id", &out.ReportIDs).Error; e != nil {
		return out, e
	}
	if len(out.ReportIDs) > 100000 {
		return out, ErrCrashHealthLimit
	}
	out.ReportIndexComplete = true
	out.Investigation = &view
	out.GroupKey = scope.GroupKey
	out.Family = scope.Family
	out.ReportCount = view.ReportCount
	out.OpenReportCount = view.OpenReportCount
	if e = scope.Notes(db, unit.ID).Select("COALESCE(MAX(id),0)").Scan(&out.FrontierID).Error; e != nil {
		return out, e
	}
	if e = scope.Notes(db, unit.ID).Where("id<=?", out.FrontierID).Count(&out.NoteCount).Error; e != nil {
		return out, e
	}
	notes := []ReportNote{}
	if e = scope.Notes(db, unit.ID).Where("id>? AND id<=?", after, out.FrontierID).Order("id ASC").Limit(limit + 1).Find(&notes).Error; e != nil {
		return out, e
	}
	out.HasMore = len(notes) > limit
	if out.HasMore {
		notes = notes[:limit]
	}
	ids := []uint{}
	for _, n := range notes {
		if n.Kind == "crash" {
			ids = append(ids, n.ReportID)
		}
	}
	reports := []struct {
		ID              uint
		Resolved        bool
		Build, GroupKey string
	}{}
	if len(ids) > 0 {
		if e = scope.Reports(db).Select("c.id,c.resolved,c.build,c.group_key").Where("c.id IN ?", ids).Scan(&reports).Error; e != nil {
			return out, e
		}
	}
	for _, n := range notes {
		item := CrashInvestigationNote{ID: n.ID, CreatedAt: n.CreatedAt, Actor: n.Actor, Note: n.Note, State: n.State, ReleaseState: n.ReleaseState, Scope: "group"}
		if n.Kind == "crash" {
			item.Scope = "report_history"
			item.ReportID = n.ReportID
			for _, r := range reports {
				if r.ID == n.ReportID {
					item.ReportResolved = r.Resolved
					item.ReportBuild = r.Build
					item.ReportGroupKey = r.GroupKey
				}
			}
		}
		out.Items = append(out.Items, item)
		out.NextAfterID = uint64(n.ID)
	}
	return out, nil
}
