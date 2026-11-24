// Read-only audit against an explicitly named disposable restored schema.
package main

import (
	"api.clyde.games/models"
	"context"
	"encoding/json"
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"time"
)

func run() error {
	dsn := os.Getenv("METRIC_ANALYSIS_AUDIT_DSN")
	cfg, err := driver.ParseDSN(dsn)
	if err != nil || cfg.DBName != "portal_metrics_rehearsal_20261116" || cfg.Net != "tcp" || cfg.Addr != "127.0.0.1:3306" {
		return fmt.Errorf("explicit disposable loopback schema required")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("audit database unavailable")
	}
	models.AnalyticsDB = db
	pool, _ := db.DB()
	defer pool.Close()
	results := map[string]any{}
	for _, f := range []models.MetricFilters{{Project: "cursemark"}, {Project: "cursemark", CreatedFrom: "2026-11-16T05:58:21Z", CreatedTo: "2026-11-17T00:00:00Z", Environment: "production"}} {
		for _, card := range models.MetricCatalog() {
			if !models.GameplayMetricAllowed(card.ID) {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			r, e := models.RunFilteredMetric(ctx, card.ID, f)
			cancel()
			if e != nil {
				return fmt.Errorf("metric %d audit failed: %w", card.ID, e)
			}
			key := fmt.Sprintf("%d/%s", card.ID, f.CreatedFrom)
			results[key] = map[string]any{"columns": r.Columns, "rows": r.Rows, "truncated": r.Truncated}
		}
	}
	return json.NewEncoder(os.Stdout).Encode(results)
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
