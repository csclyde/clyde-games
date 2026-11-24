package main

import (
	"api.clyde.games/models"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"sort"
	"strconv"
	"strings"
)

type auditRow struct {
	Columns []string `json:"columns"`
	Rows    int      `json:"rows"`
	SHA256  string   `json:"sha256"`
}

func capture(db *sql.DB, cols map[string][]string) (map[string]auditRow, error) {
	out := map[string]auditRow{}
	for _, table := range []string{"feedbacks", "crashes", "crash_groups", "savegames", "projects"} {
		fields := cols[table]
		if len(fields) == 0 {
			r, e := db.Query("SELECT COLUMN_NAME FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? ORDER BY ORDINAL_POSITION", table)
			if e != nil {
				return nil, e
			}
			for r.Next() {
				var c string
				if e := r.Scan(&c); e != nil {
					return nil, e
				}
				fields = append(fields, c)
			}
			r.Close()
			cols[table] = fields
		}
		if len(fields) == 0 {
			continue
		}
		q := []string{}
		for _, f := range fields {
			q = append(q, "`"+strings.ReplaceAll(f, "`", "``")+"`")
		}
		query := "SELECT " + strings.Join(q, ",") + " FROM `" + table + "`"
		var args []interface{}
		if limitText := os.Getenv("FEEDBACK_AUDIT_CRASH_BEFORE_ID"); table == "crashes" && limitText != "" {
			limit, err := strconv.ParseUint(limitText, 10, 64)
			if err != nil {
				return nil, err
			}
			query += " WHERE id < ?"
			args = append(args, limit)
		}
		r, e := db.Query(query, args...)
		if e != nil {
			return nil, e
		}
		hashes := []string{}
		for r.Next() {
			v := make([]sql.RawBytes, len(fields))
			p := make([]interface{}, len(v))
			for i := range v {
				p[i] = &v[i]
			}
			if e := r.Scan(p...); e != nil {
				return nil, e
			}
			b, _ := json.Marshal(v)
			h := sha256.Sum256(b)
			hashes = append(hashes, hex.EncodeToString(h[:]))
		}
		e = r.Err()
		r.Close()
		if e != nil {
			return nil, e
		}
		sort.Strings(hashes)
		h := sha256.Sum256([]byte(strings.Join(hashes, "\n")))
		out[table] = auditRow{fields, len(hashes), hex.EncodeToString(h[:])}
	}
	return out, nil
}
func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "migrate" && os.Args[1] != "fingerprint") {
		return fmt.Errorf("choose migrate or fingerprint")
	}
	dsn := os.Getenv("FEEDBACK_MIGRATION_DSN")
	if dsn == "" {
		return fmt.Errorf("FEEDBACK_MIGRATION_DSN required")
	}
	db, e := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		return fmt.Errorf("database open failed")
	}
	pool, _ := db.DB()
	defer pool.Close()
	cols := map[string][]string{}
	before, e := capture(pool, cols)
	if e != nil {
		return e
	}
	if os.Args[1] == "migrate" {
		if e := db.AutoMigrate(&models.Feedback{}, &models.SyncState{}, &models.DiscordPendingMessage{}, &models.DiscordSnapshotRevision{}, &models.DiscordSyncSource{}, &models.Project{}, &models.EventSetting{}); e != nil {
			return e
		}
		models.AnalyticsDB = db
		if e := models.MigrateFeedbackSyncConfiguration(); e != nil {
			return e
		}
		if e := models.MigrateProjectCrashCutoffs(); e != nil {
			return e
		}
	}
	after, e := capture(pool, cols)
	if e != nil {
		return e
	}
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		return fmt.Errorf("existing data changed during migration")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"before": before, "after": after, "existing_data_preserved": true, "crash_migration_run": false})
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
