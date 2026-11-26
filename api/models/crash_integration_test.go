package models

import (
	"database/sql"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Opt-in integration test: creates ONLY a randomly named database on an
// explicitly supplied, database-less loopback connection. No production DSNs.
func TestCrashMySQLMigrationConcurrencyAndRecurrence(t *testing.T) {
	dsn := os.Getenv("CRASH_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set CRASH_TEST_MYSQL_DSN for disposable loopback database test")
	}
	config, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	host, _, err := net.SplitHostPort(config.Addr)
	if err != nil || config.Net != "tcp" || config.DBName != "" || (host != "127.0.0.1" && host != "::1") {
		t.Fatal("test requires tcp loopback DSN without a database name")
	}
	admin, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("crash_redesign_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec("DROP DATABASE `" + name + "`"); err != nil {
			t.Error(err)
		}
	}()
	config.DBName, config.ParseTime = name, true
	db, err := gorm.Open(mysql.Open(config.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	pool.SetMaxOpenConns(24)
	defer pool.Close()
	old := AnalyticsDB
	AnalyticsDB = db
	defer func() { AnalyticsDB = old }()
	if err := db.AutoMigrate(&Project{}, &EventSetting{}); err != nil {
		t.Fatal(err)
	}
	// Start with the pre-redesign physical table, then exercise the real
	// additive DDL migration with history already present.
	if err := db.Exec("CREATE TABLE crashes (id bigint unsigned NOT NULL AUTO_INCREMENT PRIMARY KEY, created_at datetime(3), updated_at datetime(3), deleted_at datetime(3), p_id tinytext, project tinytext, message text, stack text, env tinytext, category tinytext, platform tinytext, build tinytext, `commit` tinytext, hash tinytext, count int, resolved boolean DEFAULT false)").Error; err != nil {
		t.Fatal(err)
	}
	created, updated := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC), time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)
	legacy := Crash{Model: gorm.Model{CreatedAt: created, UpdatedAt: updated}, Project: "cursemark", PID: "original", Message: "failure", Stack: "game.Player.update Player.hx:10\ngame.Main.tick Main.hx:40\nGPU Model: old", Platform: "windows", Build: "2020-01-01", Commit: "original", Hash: "legacy-hash", Count: 42, Resolved: true}
	legacyFields := []string{"CreatedAt", "UpdatedAt", "DeletedAt", "PID", "Project", "Message", "Stack", "Env", "Category", "Platform", "Build", "Commit", "Hash", "Count", "Resolved"}
	if err := db.Select(legacyFields).Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	// Include soft-deleted legacy history, without reviving it.
	deleted := legacy
	deleted.ID = 0
	deleted.DeletedAt = gorm.DeletedAt{Time: updated, Valid: true}
	if err := db.Select(legacyFields).Create(&deleted).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Crash{}, &CrashGroup{}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := MigrateCrashGroups(); err != nil {
			t.Fatal(err)
		}
	}
	var migrated Crash
	if err := db.First(&migrated, legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if migrated.Count != 42 || !migrated.Resolved || migrated.OccurrenceVersion != 0 || migrated.IngestedAt != nil || migrated.ResolvedAt != nil || !migrated.CreatedAt.Equal(created) || !migrated.UpdatedAt.Equal(updated) || migrated.PID != legacy.PID || migrated.Stack != legacy.Stack || migrated.Hash != legacy.Hash || migrated.Commit != legacy.Commit || migrated.Build != legacy.Build {
		t.Fatalf("legacy history changed: %+v", migrated)
	}
	var deletedAfter Crash
	if err := db.Unscoped().First(&deletedAfter, deleted.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !deletedAfter.DeletedAt.Valid || deletedAfter.GroupKey != migrated.GroupKey || deletedAfter.Count != 42 {
		t.Fatal("deleted history lost or revived")
	}

	const n = 16
	results := make(chan *Crash, n)
	failures := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := legacy
			c.PID = fmt.Sprint(i)
			c.Build = fmt.Sprintf("2026-11-%02d", i+1)
			c.Commit = fmt.Sprint(i)
			c.Stack = fmt.Sprintf("game.Player.update Player.hx:10\ngame.Main.tick Main.hx:%d\nGPU Model: gpu-%d", 50+i, i)
			got, err := AddCrash(c)
			if err != nil {
				failures <- err
				return
			}
			results <- got
		}(i)
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	ids := []uint64{}
	for c := range results {
		if c.GroupKey != migrated.GroupKey {
			t.Fatal("equivalent crash not grouped")
		}
		ids = append(ids, uint64(c.ID))
	}
	if len(ids) != n {
		t.Fatal("lost concurrent insert")
	}
	// Same hash must not cross project boundaries or conflate other callers.
	otherProject := legacy
	otherProject.Project = "necrovale"
	other, err := AddCrash(otherProject)
	if err != nil {
		t.Fatal(err)
	}
	otherCause := legacy
	otherCause.Stack = strings.ReplaceAll(legacy.Stack, "Player.update", "Menu.update")
	unrelated, err := AddCrash(otherCause)
	if err != nil {
		t.Fatal(err)
	}

	failures = make(chan error, n)
	for _, id := range ids {
		wg.Add(1)
		go func(id uint64) {
			defer wg.Done()
			_, err := ResolveCrashID(db, id)
			if err != nil {
				failures <- err
			}
		}(id)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	var members []Crash
	if err := db.Where("group_key = ?", migrated.GroupKey).Find(&members).Error; err != nil {
		t.Fatal(err)
	}
	if len(members) != n+1 {
		t.Fatalf("lost stored reports: %d", len(members))
	}
	resolutionTimes := map[uint]time.Time{}
	for _, c := range members {
		if !c.Resolved {
			t.Fatal("group member left open")
		}
		if c.OccurrenceVersion == 1 {
			if c.Count != 1 || c.ResolvedAt == nil || c.IngestedAt == nil || !c.UpdatedAt.Equal(*c.IngestedAt) {
				t.Fatalf("incorrect occurrence: %+v", c)
			}
			resolutionTimes[c.ID] = *c.ResolvedAt
		}
	}
	for _, id := range []uint{other.ID, unrelated.ID} {
		var c Crash
		db.First(&c, id)
		if c.Resolved {
			t.Fatal("unrelated group resolved")
		}
	}

	recurrence, err := AddCrash(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if recurrence.Resolved || recurrence.ID == legacy.ID {
		t.Fatal("recurrence overwrote resolved history")
	}
	var open int64
	db.Model(&Crash{}).Where("group_key = ? AND resolved = ?", migrated.GroupKey, false).Count(&open)
	if open != 1 {
		t.Fatalf("expected one new open occurrence, got %d", open)
	}
	if _, err := ResolveCrashID(db, ids[0]); err != nil {
		t.Fatal(err)
	}
	for id, at := range resolutionTimes {
		var c Crash
		db.First(&c, id)
		if !c.ResolvedAt.Equal(at) {
			t.Fatal("repeated resolution replaced timestamp")
		}
	}

	// Race a new insert against resolution through an already-resolved ID.
	// Either lock ordering is valid, but no occurrence or timestamp can be lost.
	start := make(chan struct{})
	failures = make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, err := AddCrash(legacy)
		if err != nil {
			failures <- err
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		_, err := ResolveCrashID(db, ids[1])
		if err != nil {
			failures <- err
		}
	}()
	close(start)
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	var count int64
	db.Model(&Crash{}).Where("group_key = ?", migrated.GroupKey).Count(&count)
	if count != n+3 {
		t.Fatalf("concurrent resolution/insert lost history: %d", count)
	}
	var occurrences []Crash
	db.Where("group_key = ? AND occurrence_version = 1", migrated.GroupKey).Find(&occurrences)
	seen := map[string]bool{}
	for _, c := range occurrences {
		if !c.UpdatedAt.Equal(*c.IngestedAt) || c.Count != 1 {
			t.Fatal("mutated occurrence")
		}
		if c.PID != "original" {
			seen[c.PID] = true
		}
	}
	if len(seen) != n {
		t.Fatal("concurrent per-player metadata lost")
	}
}
