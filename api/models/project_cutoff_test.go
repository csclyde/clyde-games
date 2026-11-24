package models

import (
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"testing"
)

func cutoffMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	c, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	db, e := gorm.Open(mysql.New(mysql.Config{Conn: c, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		c.Close()
		if e := m.ExpectationsWereMet(); e != nil {
			t.Error(e)
		}
	})
	return db, m
}
func TestProjectBuildCutoffIsolation(t *testing.T) {
	db, m := cutoffMock(t)
	for _, subject := range []string{"event", "savegame"} {
		m.ExpectQuery("SELECT .* FROM `event_settings`").WithArgs(projectBuildSettingKey("cursemark")).WillReturnRows(sqlmock.NewRows([]string{"key", "value"}).AddRow(projectBuildSettingKey("cursemark"), "2026-11-01"))
		if e := rejectBuildBeforeOldest(db, "2026-10-01", subject, "cursemark"); !errors.Is(e, ErrBuildTooOld) {
			t.Fatalf("expected rejection: %v", e)
		}
		m.ExpectQuery("SELECT .* FROM `event_settings`").WithArgs(projectBuildSettingKey("necrovale")).WillReturnRows(sqlmock.NewRows([]string{"key", "value"}).AddRow(projectBuildSettingKey("necrovale"), ""))
		if e := rejectBuildBeforeOldest(db, "2026-10-01", subject, "necrovale"); e != nil {
			t.Fatal(e)
		}
		m.ExpectQuery("SELECT .* FROM `event_settings`").WithArgs(projectBuildSettingKey("new-game")).WillReturnRows(sqlmock.NewRows([]string{"key", "value"}))
		if e := rejectBuildBeforeOldest(db, "2026-10-01", subject, "new-game"); e != nil {
			t.Fatal(e)
		}
	}
	if projectBuildSettingKey("") != oldestBuildSetting || settingsProject(nil) != "cursemark" {
		t.Fatal("legacy compatibility changed")
	}
	if projectBuildSettingKey("cursemark") == projectBuildSettingKey("pathprodigy") {
		t.Fatal("project key collision")
	}
}
func TestProjectCutoffMigrationAlreadyDone(t *testing.T) {
	db, m := cutoffMock(t)
	old := AnalyticsDB
	AnalyticsDB = db
	defer func() { AnalyticsDB = old }()
	m.ExpectBegin()
	m.ExpectQuery("SELECT count.* FROM `sync_states`").WithArgs("project_build_cutoffs_v1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	m.ExpectCommit()
	if e := MigrateProjectBuildCutoffs(); e != nil {
		t.Fatal(e)
	}
}

func TestProjectCutoffMigrationSnapshotsWithoutDeleting(t *testing.T) {
	db, m := cutoffMock(t)
	old := AnalyticsDB
	AnalyticsDB = db
	defer func() { AnalyticsDB = old }()
	m.ExpectBegin()
	m.ExpectQuery("SELECT count.* FROM `sync_states`").WithArgs("project_build_cutoffs_v1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	m.ExpectQuery("SELECT .* FROM `event_settings`").WithArgs(oldestBuildSetting).WillReturnRows(sqlmock.NewRows([]string{"key", "value"}).AddRow(oldestBuildSetting, "2026-11-01"))
	m.ExpectQuery("SELECT .* FROM `projects`").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("cursemark", "Cursemark").AddRow("necrovale", "Into the Necrovale"))
	m.ExpectExec("INSERT INTO `event_settings`.*ON DUPLICATE KEY UPDATE").WithArgs(projectBuildSettingKey("cursemark"), "2026-11-01").WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectExec("INSERT INTO `event_settings`.*ON DUPLICATE KEY UPDATE").WithArgs(projectBuildSettingKey("necrovale"), "2026-11-01").WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectExec("INSERT INTO `sync_states`").WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectCommit()
	if e := MigrateProjectBuildCutoffs(); e != nil {
		t.Fatal(e)
	}
}
