package endpoints

import (
	"api.clyde.games/models"
	"database/sql"
	"errors"
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net"
	"os"
	"testing"
	"time"
)

func TestFeedbackCheckpointMySQLFirstFailureAndRetry(t *testing.T) {
	dsn := os.Getenv("FEEDBACK_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires disposable loopback DSN")
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
	name := fmt.Sprintf("feedback_checkpoint_test_%d", time.Now().UnixNano())
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
	old := models.AnalyticsDB
	models.AnalyticsDB = db
	defer func() { models.AnalyticsDB = old }()
	if e := db.AutoMigrate(&models.SyncState{}); e != nil {
		t.Fatal(e)
	}
	p := models.Project{ID: "cursemark"}
	_, _, from, _, e := runFeedbackSource(p, "discord", "", func(a, b time.Time) (int, int, error) { return 2, 0, errors.New("incomplete") })
	if e == nil {
		t.Fatal("failure hidden")
	}
	s, e := models.GetSyncState("discord_cursemark")
	if e != nil || s.Status != "failed" || !s.LastSuccessAt.IsZero() {
		t.Fatalf("first failure %+v %v", s, e)
	}
	_, _, retryFrom, to, e := runFeedbackSource(p, "discord", "", func(a, b time.Time) (int, int, error) { return 0, 2, nil })
	if e != nil || retryFrom.Sub(from) > time.Millisecond || from.Sub(retryFrom) > time.Millisecond {
		t.Fatalf("retry window %s %s %v", from, retryFrom, e)
	}
	s, e = models.GetSyncState("discord_cursemark")
	if e != nil || s.Status != "success" || s.LastSuccessAt.Sub(to) > time.Millisecond || to.Sub(s.LastSuccessAt) > time.Millisecond {
		t.Fatalf("success %+v %v", s, e)
	}
}
