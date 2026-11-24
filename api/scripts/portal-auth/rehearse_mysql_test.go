package main

import (
	"database/sql"
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

func TestPortalRehearsalMySQLPreservesOriginalRowsAndSchema(t *testing.T) {
	dsn := os.Getenv("FEEDBACK_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires disposable database-less loopback DSN")
	}
	cfg, e := driver.ParseDSN(dsn)
	if e != nil {
		t.Fatal("invalid test DSN")
	}
	host, _, e := net.SplitHostPort(cfg.Addr)
	if e != nil || cfg.Net != "tcp" || cfg.DBName != "" || (host != "127.0.0.1" && host != "::1") {
		t.Fatal("database-less loopback test DSN required")
	}
	admin, e := sql.Open("mysql", dsn)
	if e != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	name := fmt.Sprintf("portal_auth_rehearsal_test_%d", time.Now().UnixNano())
	if _, e = admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4"); e != nil {
		t.Fatal("cannot create disposable database")
	}
	defer admin.Exec("DROP DATABASE `" + name + "`")
	cfg.DBName = name
	cfg.ParseTime = true
	db, e := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal("cannot open disposable database")
	}
	pool, _ := db.DB()
	defer pool.Close()
	for _, q := range []string{
		"CREATE TABLE feedback (id BIGINT PRIMARY KEY,resolved BOOLEAN NOT NULL,message TEXT,created_at DATETIME(6))",
		"INSERT INTO feedback VALUES(1,1,'handled fixture','2020-01-01'),(2,0,'open fixture','2026-11-17')",
		"CREATE TABLE crashes (id BIGINT PRIMARY KEY,resolved BOOLEAN,count BIGINT,occurrence_version INT,ingested_at DATETIME(6),payload BLOB)",
		"INSERT INTO crashes VALUES(1,1,7,0,NULL,X'000102'),(2,0,1,1,'2026-11-17 00:00:00.123456',X'FF00')",
		"CREATE TABLE raw_evidence (body BLOB)", "INSERT INTO raw_evidence VALUES(NULL),(X'0000'),(X'0000')",
	} {
		if db.Exec(q).Error != nil {
			t.Fatal("fixture setup failed")
		}
	}
	before, tables, rows, e := preservedFingerprint(db)
	if e != nil || tables != 3 || rows != 7 {
		t.Fatal("fingerprint fixture failed")
	}
	for i := 0; i < 2; i++ {
		if rehearse(db, name) != nil {
			t.Fatal("repeated additive rehearsal failed")
		}
	}
	after, n, r, e := preservedFingerprint(db)
	if e != nil || after != before || n != tables || r != rows {
		t.Fatal("original schema/rows/resolution/history changed")
	}
}
