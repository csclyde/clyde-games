// Additive notes-only schema operation. DSN is supplied privately; never printed.
package main

import (
	"api.clyde.games/models"
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
)

func main() {
	cfg, e := driver.ParseDSN(os.Getenv("REPORT_NOTES_MIGRATION_DSN"))
	if e != nil || cfg == nil || cfg.DBName == "" {
		panic("explicit notes migration DSN required")
	}
	db, e := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		panic("notes migration database unavailable")
	}
	if e = models.MigrateReportNotes(db); e != nil {
		panic("notes migration failed")
	}
	fmt.Println(`{"notes_schema_ready":true,"existing_records_modified":false}`)
}
