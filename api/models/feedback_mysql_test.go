package models

import (
	"database/sql"
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

func TestFeedbackMySQLDurableConcurrentIdentity(t *testing.T) {
	dsn := os.Getenv("FEEDBACK_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires disposable loopback test DSN")
	}
	cfg, e := driver.ParseDSN(dsn)
	if e != nil {
		t.Fatal(e)
	}
	h, _, e := net.SplitHostPort(cfg.Addr)
	if e != nil || cfg.Net != "tcp" || cfg.DBName != "" || (h != "127.0.0.1" && h != "::1") {
		t.Fatal("database-less loopback DSN required")
	}
	admin, e := sql.Open("mysql", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	name := fmt.Sprintf("feedback_ingestion_test_%d", time.Now().UnixNano())
	if _, e := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4"); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec("DROP DATABASE `" + name + "`")
	cfg.DBName = name
	cfg.ParseTime = true
	db, e := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	pool, _ := db.DB()
	defer pool.Close()
	pool.SetMaxOpenConns(20)
	old := AnalyticsDB
	AnalyticsDB = db
	defer func() { AnalyticsDB = old }()
	if e := db.AutoMigrate(&Feedback{}, &Project{}, &SyncState{}, &DiscordPendingMessage{}); e != nil {
		t.Fatal(e)
	}
	created := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
	fb := Feedback{Project: "cursemark", Message: "original\nhttps://test.invalid/topic/", Category: "Steam Discussion", Platform: "Steam", Resolved: true}
	fb.CreatedAt = created
	fb.UpdatedAt = created
	if e := db.Create(&fb).Error; e != nil {
		t.Fatal(e)
	}
	run := func(f Feedback, id, url string) {
		var wg sync.WaitGroup
		errs := make(chan error, 20)
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _, e := AddSourceFeedback(f, "steam_discussions", id, url); errs <- e }()
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			if e != nil {
				t.Error(e)
			}
		}
	}
	run(fb, "topic", "https://test.invalid/topic/")
	var rows []Feedback
	db.Unscoped().Find(&rows)
	if len(rows) != 1 || !rows[0].Resolved || rows[0].Message != fb.Message || !rows[0].CreatedAt.Equal(created) || !rows[0].UpdatedAt.Equal(created) {
		t.Fatalf("legacy changed %+v", rows)
	}
	if e := db.Delete(&rows[0]).Error; e != nil {
		t.Fatal(e)
	}
	if added, e := AddSourceFeedback(fb, "steam_discussions", "topic", "https://test.invalid/topic/"); e != nil || added {
		t.Fatalf("deleted replay %v %v", added, e)
	}
	fresh := fb
	fresh.ID = 0
	fresh.Resolved = false
	fresh.Message = "new\nhttps://test.invalid/new/"
	run(fresh, "new", "https://test.invalid/new/")
	var n int64
	db.Unscoped().Model(&Feedback{}).Count(&n)
	if n != 2 {
		t.Fatalf("duplicate intake %d", n)
	}
	for i := 0; i < 2; i++ {
		if e := db.AutoMigrate(&Feedback{}, &SyncState{}, &DiscordPendingMessage{}); e != nil {
			t.Fatal(e)
		}
	}
}
