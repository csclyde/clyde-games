package endpoints

import (
	"api.clyde.games/models"
	"encoding/json"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestCrashHealthMySQLExactScopeAndReadOnlyHistory(t *testing.T) {
	db := syncTestDB(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	closed := now.Add(-2 * time.Hour)
	earlier := now.Add(-time.Hour)
	rows := []models.Crash{
		{Project: "cursemark", Message: "Access violation", GroupKey: "g", Count: 17, Resolved: true, ResolvedAt: &closed, PID: "legacy", Build: "2020"},
		{Project: "cursemark", Message: "Access violation", GroupKey: "g", Count: 1, OccurrenceVersion: 1, IngestedAt: &earlier, PID: "one", Build: "2026-11-16"},
		{Project: "cursemark", Message: "Access violation", GroupKey: "g", Count: 1, OccurrenceVersion: 1, IngestedAt: &now, PID: "one", Build: "2026-11-16"},
		{Project: "necrovale", Message: "Access violation", GroupKey: "g", Count: 1, OccurrenceVersion: 1, IngestedAt: &now, PID: "one", Build: "2026-11-16"},
		{Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: now, Valid: true}}, Project: "cursemark", GroupKey: "g", Count: 1, OccurrenceVersion: 1, IngestedAt: &now, PID: "deleted", Build: "2026-11-16"},
	}
	if e := db.Create(&rows).Error; e != nil {
		t.Fatal(e)
	}
	unknown := models.Crash{Project: "cursemark", GroupKey: "unknown-resolution", Count: 3}
	if e := db.Create(&unknown).Error; e != nil {
		t.Fatal(e)
	}
	if e := db.Model(&unknown).UpdateColumn("resolved", nil).Error; e != nil {
		t.Fatal(e)
	}
	var before, after []models.Crash
	db.Unscoped().Order("id").Find(&before)
	fingerprint, _ := json.Marshal(before)
	h, e := models.SelectCrashHealth(db, "cursemark", 7, now)
	if e != nil {
		t.Fatal(e)
	}
	if h.StoredRows != 4 || h.Reports != 2 || h.ReportingIDs != 1 || h.LegacyOccurrences != 20 || h.ReturnedReports != 2 || h.ReturnedOpenGroups != 1 || h.OpenGroups != 1 || len(h.Families) != 1 || h.Families[0].ReturnedReportingIDs != 1 {
		t.Fatalf("scoped %+v", h)
	}
	all, e := models.SelectCrashHealth(db, "all", 30, now)
	if e != nil || all.Reports != 3 || all.ReportingIDs != 2 || all.ReturnedOpenGroups != 1 {
		t.Fatalf("all %+v %v", all, e)
	}
	db.Unscoped().Order("id").Find(&after)
	next, _ := json.Marshal(after)
	if string(fingerprint) != string(next) {
		t.Fatal("read changed original rows")
	}
}
