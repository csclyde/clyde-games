package models

import (
	"crypto/rand"
	"fmt"
	"io"
	"strings"

	"gorm.io/gorm"
)

var necrovaleColumns = []struct{ name, definition string }{
	{"hopestones", "DOUBLE NULL"}, {"gold", "DOUBLE NULL"},
	{"lifetime_clears", "BIGINT UNSIGNED NULL"}, {"lifetime_deaths", "BIGINT UNSIGNED NULL"},
	{"clear_history_complete", "BOOLEAN NULL"}, {"death_history_complete", "BOOLEAN NULL"},
}

var cursemarkColumns = []string{
	"cursemark", "resource_essence", "resource_tears", "action_attack",
	"action_spell", "action_ward", "action_ult", "action_botyl",
}

// MigrateGameMetrics prepares new tables, verifies the copies, and switches all
// table names atomically. The caller must drain and stop every event writer for
// the duration. Original tables are retained under the printed archive suffix.
func MigrateGameMetrics(db *gorm.DB, cursemark, necrovale string, rollback bool, out io.Writer) error {
	shared := metricStore{schema: db.Migrator().CurrentDatabase()}
	stores, err := newGameMetricStores(cursemark, necrovale, shared.schema)
	if err != nil {
		return err
	}
	return db.Connection(func(db *gorm.DB) error {
		db = db.Session(&gorm.Session{NewDB: true})
		var held int
		lock := "clyde_game_metrics_" + shared.schema
		if len(lock) > 64 {
			return fmt.Errorf("shared schema name is too long for the migration lock")
		}
		if err := db.Raw("SELECT GET_LOCK(?, 0)", lock).Scan(&held).Error; err != nil || held != 1 {
			return fmt.Errorf("unable to acquire metrics migration lock")
		}
		defer db.Exec("DO RELEASE_LOCK(?)", lock)
		hasSplit := db.Migrator().HasTable("game_metrics_split")
		if hasSplit != rollback {
			return fmt.Errorf("the requested migration does not match the active storage layout")
		}
		if rollback {
			var count int64
			if err := db.Raw("SELECT COUNT(*) FROM game_metrics_split WHERE id = 1 AND cursemark = ? AND necrovale = ?", cursemark, necrovale).Scan(&count).Error; err != nil || count != 1 {
				return fmt.Errorf("active split uses different metrics schemas")
			}
		}
		var nonce [6]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return err
		}
		suffix := fmt.Sprintf("_%x", nonce)
		archive := metricStore{schema: "game_metrics_archive" + suffix}
		fmt.Fprintf(out, "Game metrics archive database %s\n", archive.schema)
		if err := db.Exec("CREATE DATABASE " + quoteIdentifier(archive.schema)).Error; err != nil {
			return err
		}
		if rollback {
			return rollbackGameMetrics(db, shared, archive, stores, suffix, out)
		}
		return splitGameMetrics(db, shared, archive, stores, suffix, out)
	})
}

func splitGameMetrics(db *gorm.DB, shared, archive metricStore, stores []metricStore, suffix string, out io.Writer) error {
	scopes := []string{
		"NOT (LOWER(TRIM(COALESCE(e.project, ''))) IN ('', 'cursemark', 'necrovale'))",
		"LOWER(TRIM(COALESCE(e.project, ''))) IN ('', 'cursemark')",
		"LOWER(TRIM(COALESCE(e.project, ''))) = 'necrovale'",
	}
	renames := archiveMetricTables(shared, archive, "shared")
	for i, store := range append([]metricStore{shared}, stores...) {
		if err := db.Exec("CREATE DATABASE IF NOT EXISTS " + quoteIdentifier(store.schema)).Error; err != nil {
			return err
		}
		if err := createMetricTables(db, shared, store, suffix, store.project == necrovaleProject); err != nil {
			return err
		}
		if err := copyMetricTables(db, shared, store, suffix, scopes[i], i == 0, true, out); err != nil {
			return err
		}
		constraint := []string{"fk_other_events_items", "fk_cursemark_events_items", "fk_necrovale_events_items"}[i]
		if err := addMetricItemsConstraint(db, store, suffix, constraint); err != nil {
			return err
		}
		renames = append(renames, activateMetricTables(store, suffix)...)
	}
	stage := shared.sqlTable("gm_sequence" + suffix)
	if err := db.Exec("CREATE TABLE " + stage + " (id INT PRIMARY KEY, next_id BIGINT UNSIGNED NOT NULL, cursemark VARCHAR(64) NOT NULL, necrovale VARCHAR(64) NOT NULL) ENGINE=InnoDB").Error; err != nil {
		return err
	}
	if err := db.Exec("INSERT INTO "+stage+" SELECT 1, GREATEST(COALESCE(MAX(id), 0), COALESCE((SELECT AUTO_INCREMENT - 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = 'events'), 0)), ?, ? FROM "+shared.sqlTable("events"), shared.schema, stores[0].schema, stores[1].schema).Error; err != nil {
		return err
	}
	renames = append(renames, stage+" TO "+shared.sqlTable("game_metrics_split"))
	return db.Exec("RENAME TABLE " + strings.Join(renames, ", ")).Error
}

func rollbackGameMetrics(db *gorm.DB, shared, archive metricStore, stores []metricStore, suffix string, out io.Writer) error {
	if err := createMetricTables(db, shared, shared, suffix, false); err != nil {
		return err
	}
	if err := addNecrovaleColumns(db, shared.sqlTable("gm_events"+suffix), nil); err != nil {
		return err
	}
	renames := archiveMetricTables(shared, archive, "shared")
	for _, store := range append([]metricStore{shared}, stores...) {
		if err := copyMetricTables(db, store, shared, suffix, "1 = 1", true, false, out); err != nil {
			return err
		}
		if store.project != "" {
			renames = append(renames, archiveMetricTables(store, archive, store.project)...)
		}
	}
	if err := addMetricItemsConstraint(db, shared, suffix, "fk_events_items"); err != nil {
		return err
	}
	var nextID uint64
	if err := db.Raw("SELECT next_id + 1 FROM " + shared.sqlTable("game_metrics_split") + " WHERE id = 1").Scan(&nextID).Error; err != nil {
		return err
	}
	if err := db.Exec(fmt.Sprintf("ALTER TABLE %s AUTO_INCREMENT = %d", shared.sqlTable("gm_events"+suffix), nextID)).Error; err != nil {
		return err
	}
	renames = append(renames, activateMetricTables(shared, suffix)...)
	renames = append(renames, shared.sqlTable("game_metrics_split")+" TO "+archive.sqlTable("split_sequence"))
	return db.Exec("RENAME TABLE " + strings.Join(renames, ", ")).Error
}

func archiveMetricTables(store, archive metricStore, label string) []string {
	return []string{store.sqlTable("event_items") + " TO " + archive.sqlTable(label+"_items"),
		store.sqlTable("events") + " TO " + archive.sqlTable(label+"_events")}
}

func activateMetricTables(store metricStore, suffix string) []string {
	return []string{store.sqlTable("gm_events"+suffix) + " TO " + store.sqlTable("events"),
		store.sqlTable("gm_items"+suffix) + " TO " + store.sqlTable("event_items")}
}

func addMetricItemsConstraint(db *gorm.DB, store metricStore, suffix, name string) error {
	return db.Exec("ALTER TABLE " + store.sqlTable("gm_items"+suffix) + " ADD CONSTRAINT " +
		quoteIdentifier(name) + " FOREIGN KEY (event_id) REFERENCES " + store.sqlTable("gm_events"+suffix) + " (id)").Error
}

func createMetricTables(db *gorm.DB, source, dest metricStore, suffix string, necrovale bool) error {
	events := dest.sqlTable("gm_events" + suffix)
	items := dest.sqlTable("gm_items" + suffix)
	if err := db.Exec("CREATE TABLE " + events + " LIKE " + source.sqlTable("events")).Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE TABLE " + items + " LIKE " + source.sqlTable("event_items")).Error; err != nil {
		return err
	}
	// The baseline archive retains the original columns, constraints and data.
	// Each active schema has the fields applicable to that game.
	columns, err := metricTableColumns(db, dest.schema, "gm_events"+suffix)
	if err != nil {
		return err
	}
	if necrovale {
		// A payload carrying real Cursemark-specific values should be reviewed
		// before cutover, rather than silently dropping historical metrics.
		conditions := make([]string, 0, len(cursemarkColumns))
		for _, column := range cursemarkColumns {
			conditions = append(conditions, "COALESCE("+quoteIdentifier(column)+", 0) <> 0")
		}
		var mismatched int64
		if err := db.Raw("SELECT COUNT(*) FROM " + source.sqlTable("events") +
			" WHERE LOWER(TRIM(project)) = 'necrovale' AND (" + strings.Join(conditions, " OR ") + ")").Scan(&mismatched).Error; err != nil {
			return err
		}
		if mismatched != 0 {
			return fmt.Errorf("%d historical Necrovale events contain Cursemark-specific values", mismatched)
		}
		for _, column := range cursemarkColumns {
			if err := db.Exec("ALTER TABLE " + events + " DROP COLUMN " + quoteIdentifier(column)).Error; err != nil {
				return err
			}
		}
		return addNecrovaleColumns(db, events, columns)
	}
	for _, column := range necrovaleColumns {
		if columns[column.name] {
			if err := db.Exec("ALTER TABLE " + events + " DROP COLUMN " + quoteIdentifier(column.name)).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func addNecrovaleColumns(db *gorm.DB, table string, existing map[string]bool) error {
	parts := make([]string, 0, len(necrovaleColumns))
	for _, column := range necrovaleColumns {
		if !existing[column.name] {
			parts = append(parts, "ADD COLUMN "+quoteIdentifier(column.name)+" "+column.definition)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return db.Exec("ALTER TABLE " + table + " " + strings.Join(parts, ", ")).Error
}

func metricTableColumns(db *gorm.DB, schema, table string) (map[string]bool, error) {
	var columns []string
	err := db.Raw("SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION", schema, table).Scan(&columns).Error
	result := make(map[string]bool, len(columns))
	for _, column := range columns {
		result[column] = true
	}
	return result, err
}

func copyMetricTables(db *gorm.DB, source, dest metricStore, suffix, scope string, keepOrphans, split bool, out io.Writer) error {
	for _, table := range []struct{ original, stage, alias string }{{"events", "gm_events", "e"}, {"event_items", "gm_items", "i"}} {
		from := source.sqlTable(table.original) + " " + table.alias
		where := scope
		if table.alias == "i" {
			from += " LEFT JOIN " + source.sqlTable("events") + " e ON e.id = i.event_id"
			if keepOrphans {
				where = "(" + where + " OR e.id IS NULL)"
			}
		}
		from += " WHERE " + where
		target := dest.sqlTable(table.stage + suffix)
		sourceColumns, err := metricTableColumns(db, source.schema, table.original)
		if err != nil {
			return err
		}
		var targetColumns []string
		if err := db.Raw("SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION", dest.schema, table.stage+suffix).Scan(&targetColumns).Error; err != nil {
			return err
		}
		var names, values []string
		for _, column := range targetColumns {
			if sourceColumns[column] {
				names = append(names, quoteIdentifier(column))
				value := table.alias + "." + quoteIdentifier(column)
				if split && dest.project != "" && column == "project" {
					value = "CASE WHEN TRIM(COALESCE(e.project, '')) = '' THEN e.project ELSE '" + dest.project + "' END"
				}
				values = append(values, value)
			}
		}
		if len(names) == 0 {
			return fmt.Errorf("no compatible columns for %s", table.original)
		}
		result := db.Exec("INSERT INTO " + target + " (" + strings.Join(names, ", ") + ") SELECT " + strings.Join(values, ", ") + " FROM " + from)
		if result.Error != nil {
			return result.Error
		}
		var expected int64
		if err := db.Raw("SELECT COUNT(*) FROM " + from).Scan(&expected).Error; err != nil {
			return err
		}
		if result.RowsAffected != expected {
			return fmt.Errorf("%s to %s %s copy mismatch: %d of %d", source.schema, dest.schema, table.original, result.RowsAffected, expected)
		}
		fmt.Fprintf(out, "%s to %s %s: %d rows\n", source.schema, dest.schema, table.original, expected)
	}
	return nil
}
