package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"sort"
)

// Read-only complete compact group index; individual occurrences are evidence.
// Stable keys coordinate work even when the displayed anchor changes.
type CrashWorkIndex struct {
	Items         []CrashWorkView      `json:"items"`
	ActiveGroups  []CrashWorkReference `json:"active_groups"`
	ActiveIDs     []uint               `json:"active_ids"`
	ActiveCount   int                  `json:"active_count"`
	IndexComplete bool                 `json:"index_complete"`
	Truncated     bool                 `json:"truncated"`
	Limit         int                  `json:"limit"`
	UnitType      string               `json:"unit_type"`
}
type CrashWorkReference struct {
	Key      string `json:"key"`
	Project  string `json:"project"`
	AnchorID uint   `json:"anchor_id"`
}

func ListCrashWork(db *gorm.DB, project string) (CrashWorkIndex, error) {
	out := CrashWorkIndex{Items: []CrashWorkView{}, ActiveGroups: []CrashWorkReference{}, ActiveIDs: []uint{}, IndexComplete: true, Limit: 200, UnitType: "crash_group"}
	var rows []struct {
		ID                uint
		Project, GroupKey string
		AccessViolation   bool
		Resolved          bool
	}
	q := db.Table("crashes").Select("id,project,group_key,(LOWER(message) LIKE '%access violation%') AS access_violation,resolved").Where("deleted_at IS NULL")
	if project != "all" {
		if project != NormalizeProjectID(project) || len(project) > 64 {
			return out, ErrNoteInput
		}
		q = q.Where("(BINARY COALESCE(TRIM(project),'')=? OR (?='cursemark' AND LOWER(TRIM(project))='cursemark'))", project, project)
	}
	if e := q.Order("id ASC").Limit(100001).Find(&rows).Error; e != nil {
		return out, e
	}
	if len(rows) > 100000 {
		return out, ErrCrashHealthLimit
	}
	scopes := map[string]CrashWorkScope{}
	active := map[string]bool{}
	for _, r := range rows {
		p := NormalizeProjectID(r.Project)
		identity := "group:" + r.GroupKey
		family := ""
		key := r.GroupKey
		if r.AccessViolation {
			family = "access_violations"
			key = ""
			identity = "family:access_violations"
		} else if r.GroupKey == "" {
			identity = fmt.Sprintf("legacy-report:%d", r.ID)
		}
		raw, _ := json.Marshal([]string{"crash-investigation-v1", p, family, identity})
		h := sha256.Sum256(raw)
		unit := hex.EncodeToString(h[:])
		if !r.Resolved {
			active[unit] = true
			if _, ok := scopes[unit]; !ok {
				scopes[unit] = CrashWorkScope{Project: p, GroupKey: key, Family: family, Key: unit, AnchorID: r.ID}
			}
		}
	}
	if len(active) > 10000 {
		return out, ErrCrashHealthLimit
	}
	list := []CrashWorkScope{}
	for _, s := range scopes {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].AnchorID < list[j].AnchorID })
	for i, s := range list {
		out.ActiveIDs = append(out.ActiveIDs, s.AnchorID)
		out.ActiveGroups = append(out.ActiveGroups, CrashWorkReference{Key: s.Key, Project: s.Project, AnchorID: s.AnchorID})
		if i < out.Limit {
			u, e := s.Unit(db, false)
			if e != nil {
				return out, e
			}
			v, e := s.View(db, u)
			if e != nil {
				return out, e
			}
			out.Items = append(out.Items, v)
		}
	}
	out.ActiveCount = len(list)
	out.Truncated = len(list) > out.Limit
	return out, nil
}
