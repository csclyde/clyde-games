package models

import (
	"gorm.io/gorm"
	"strings"
	"time"
)

// Derived from the append-only per-record log. Resolution is deliberately absent:
// ready_for_review is a handoff to the owner, not a request to close anything.
type InvestigationStatus struct {
	ID           uint       `json:"id"`
	State        string     `json:"state"`
	StateAt      *time.Time `json:"state_at,omitempty"`
	ReleaseState string     `json:"release_state,omitempty"`
	LastNoteAt   *time.Time `json:"last_note_at,omitempty"`
	NoteCount    int64      `json:"note_count"`
}

func CanonicalInvestigationState(state string) string {
	if state == "fixed" {
		return "ready_for_review"
	} // Preserve original historical notes unchanged.
	switch state {
	case "investigating", "blocked", "needs_confirmation", "ready_for_review":
		return state
	default:
		return "unreviewed"
	}
}
func ReadInvestigationStatuses(db *gorm.DB, kind, project string, ids []uint, frontier *uint64) ([]InvestigationStatus, error) {
	if len(ids) == 0 || len(ids) > 200 {
		return nil, ErrNoteInput
	}
	source, err := NoteReportQuery(db, kind)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID      uint
		Project string
	}
	if err = source.Select("id,project").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, gorm.ErrRecordNotFound
	}
	for _, row := range rows {
		if NormalizeProjectID(row.Project) != project {
			return nil, gorm.ErrRecordNotFound
		}
	}
	type latest struct {
		ReportID  uint
		StateID   uint
		ReleaseID uint
		LastID    uint
		NoteCount int64
	}
	var groups []latest
	q := db.Model(&ReportNote{}).Where("kind = ? AND BINARY project = ? AND report_id IN ?", kind, project, ids)
	if frontier != nil {
		q = q.Where("id <= ?", *frontier)
	}
	if err = q.Select("report_id, MAX(id) AS last_id, MAX(CASE WHEN state <> '' THEN id ELSE 0 END) AS state_id, MAX(CASE WHEN release_state <> '' THEN id ELSE 0 END) AS release_id, COUNT(*) AS note_count").Group("report_id").Scan(&groups).Error; err != nil {
		return nil, err
	}
	wanted := []uint{}
	for _, g := range groups {
		wanted = append(wanted, g.LastID)
		if g.StateID > 0 {
			wanted = append(wanted, g.StateID)
		}
		if g.ReleaseID > 0 {
			wanted = append(wanted, g.ReleaseID)
		}
	}
	found := []ReportNote{}
	if len(wanted) > 0 {
		if err = db.Select("id,created_at,state,release_state").Where("id IN ?", wanted).Find(&found).Error; err != nil {
			return nil, err
		}
	}
	byID := map[uint]ReportNote{}
	for _, n := range found {
		byID[n.ID] = n
	}
	byReport := map[uint]InvestigationStatus{}
	for _, g := range groups {
		last := byID[g.LastID]
		s := InvestigationStatus{ID: g.ReportID, State: "unreviewed", NoteCount: g.NoteCount, LastNoteAt: &last.CreatedAt}
		if g.StateID > 0 {
			n := byID[g.StateID]
			s.State = CanonicalInvestigationState(n.State)
			s.StateAt = &n.CreatedAt
		}
		if g.ReleaseID > 0 {
			s.ReleaseState = strings.TrimSpace(byID[g.ReleaseID].ReleaseState)
		}
		byReport[g.ReportID] = s
	}
	out := make([]InvestigationStatus, 0, len(ids))
	for _, id := range ids {
		s, ok := byReport[id]
		if !ok {
			s = InvestigationStatus{ID: id, State: "unreviewed"}
		}
		out = append(out, s)
	}
	return out, nil
}
