package main

import (
	"api.clyde.games/models"
	"encoding/json"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"time"
)

func run() error {
	dsn := os.Getenv("CRASH_HEALTH_AUDIT_DSN")
	if dsn == "" {
		return fmt.Errorf("CRASH_HEALTH_AUDIT_DSN required")
	}
	db, e := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		return fmt.Errorf("database open failed")
	}
	pool, _ := db.DB()
	defer pool.Close()
	scopes := []string{"all", "cursemark", "necrovale", "packrat", "pathprodigy"}
	out := map[string]interface{}{}
	for _, scope := range scopes {
		h, e := models.SelectCrashHealth(db, scope, 30, time.Now().UTC())
		if e != nil {
			return e
		}
		out[scope] = map[string]interface{}{"stored_rows": h.StoredRows, "reports": h.Reports, "reporting_ids": h.ReportingIDs, "missing_pid_reports": h.MissingPIDReports, "detailed_all_time": h.DetailedAllTime, "legacy_rows": h.LegacyRows, "legacy_occurrences": h.LegacyOccurrences, "open_groups": h.OpenGroups, "returned_open_groups": h.ReturnedOpenGroups, "returned_reports": h.ReturnedReports, "first_received": h.FirstReceived, "last_received": h.LastReceived, "build_count": len(h.Builds)}
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
