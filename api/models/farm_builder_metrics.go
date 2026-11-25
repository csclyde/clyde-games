package models

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const farmBuilderSeasonCompleted = "season_completed"

var ErrInvalidFarmBuilderSeason = errors.New("invalid Farm Builder season snapshot")

func farmBuilderMetricStore(db *gorm.DB, schema string) (metricStore, error) {
	if !validSchemaName(schema) {
		return metricStore{}, fmt.Errorf("invalid Farm Builder metrics schema")
	}
	var shared struct{ Cursemark, Necrovale string }
	result := db.Raw("SELECT cursemark, necrovale FROM game_metrics_split WHERE id = 1").Scan(&shared)
	if result.Error != nil {
		return metricStore{}, result.Error
	}
	if result.RowsAffected != 1 {
		return metricStore{}, fmt.Errorf("activate the Cursemark and Necrovale split first")
	}
	for _, other := range []string{db.Migrator().CurrentDatabase(), shared.Cursemark, shared.Necrovale} {
		if strings.EqualFold(other, schema) {
			return metricStore{}, fmt.Errorf("Farm Builder requires its own metrics database")
		}
	}
	return metricStore{schema, FarmBuilderProject}, nil
}

// ProvisionFarmBuilderMetrics is additive. Stop legacy event writers before
// running it, so old software cannot create Farm Builder events in analytics.
func ProvisionFarmBuilderMetrics(db *gorm.DB, schema string) error {
	store, err := farmBuilderMetricStore(db, schema)
	if err != nil {
		return err
	}
	var legacy int64
	if err := db.Raw("SELECT COUNT(*) FROM events WHERE LOWER(TRIM(project)) = ?", FarmBuilderProject).Scan(&legacy).Error; err != nil {
		return err
	}
	if legacy != 0 {
		return fmt.Errorf("found %d Farm Builder events in shared storage; preserve and migrate them before activation", legacy)
	}
	if err := db.Exec("CREATE DATABASE IF NOT EXISTS " + quoteIdentifier(schema)).Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE TABLE IF NOT EXISTS " + store.sqlTable("events") + ` (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		created_at DATETIME(3), updated_at DATETIME(3), deleted_at DATETIME(3),
		p_id VARCHAR(32) COLLATE utf8mb4_bin NOT NULL,
		s_id CHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
		env VARCHAR(32), build VARCHAR(32), project VARCHAR(32) NOT NULL,
		type VARCHAR(32) NOT NULL, level VARCHAR(128) NOT NULL, run_step BIGINT NOT NULL,
		commit VARCHAR(64), platform VARCHAR(32), gold DOUBLE NOT NULL,
		UNIQUE KEY farm_builder_season_receipt (p_id, s_id, run_step),
		KEY farm_builder_events_created_at (created_at),
		KEY farm_builder_events_build (build)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`).Error; err != nil {
		return err
	}
	return db.Exec("CREATE TABLE IF NOT EXISTS " + store.sqlTable("event_items") + ` (
		event_id BIGINT UNSIGNED NOT NULL,
		item VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
		slot CHAR(16) NOT NULL, level INT NOT NULL,
		PRIMARY KEY (event_id, slot, item),
		FOREIGN KEY (event_id) REFERENCES ` + store.sqlTable("events") + ` (id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`).Error
}

func ConfigureFarmBuilderMetrics(schema string) (func(), error) {
	store, err := farmBuilderMetricStore(AnalyticsDB, schema)
	if err != nil {
		return nil, err
	}
	if len(gameMetricStores) != 2 {
		return nil, fmt.Errorf("configure the Cursemark and Necrovale stores first")
	}
	for _, table := range []string{"events", "event_items"} {
		if err := AnalyticsDB.Exec("SELECT 1 FROM " + store.sqlTable(table) + " LIMIT 0").Error; err != nil {
			return nil, fmt.Errorf("Farm Builder metrics unavailable: %w", err)
		}
	}
	var receiptIndex int
	if err := AnalyticsDB.Raw("SELECT COUNT(*) FROM information_schema.STATISTICS WHERE table_schema = ? AND table_name = 'events' AND index_name = 'farm_builder_season_receipt' AND non_unique = 0 AND column_name IN ('p_id','s_id','run_step')", schema).Scan(&receiptIndex).Error; err != nil || receiptIndex != 3 {
		return nil, fmt.Errorf("Farm Builder season receipt index unavailable")
	}
	previous := gameMetricStores
	gameMetricStores = append(gameMetricStores, store)
	return func() { gameMetricStores = previous }, nil
}

func validateFarmBuilderSeason(event Event) error {
	if event.Type != farmBuilderSeasonCompleted || strings.TrimSpace(event.PID) == "" || !hex32Pattern.MatchString(event.SID) ||
		event.RunStep == nil || *event.RunStep < 1 || event.Gold == nil || *event.Gold < 0 || math.IsNaN(*event.Gold) || math.IsInf(*event.Gold, 0) {
		return fmt.Errorf("%w: expected season_completed, PID, 32-character lowercase hexadecimal SID, positive RunStep, and finite nonnegative Gold", ErrInvalidFarmBuilderSeason)
	}
	switch event.Level {
	case "spring", "summer", "fall", "winter":
	default:
		return fmt.Errorf("%w: Level must be spring, summer, fall, or winter", ErrInvalidFarmBuilderSeason)
	}
	seen := make(map[[2]string]struct{}, len(event.Items))
	for _, item := range event.Items {
		if (item.Slot != "piece" && item.Slot != "feature") || strings.TrimSpace(item.Item) == "" || item.Level < 1 || item.Level > math.MaxInt32 {
			return fmt.Errorf("%w: each board item requires an ID, piece or feature slot, and positive quantity", ErrInvalidFarmBuilderSeason)
		}
		key := [2]string{item.Slot, item.Item}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%w: repeat board items must be combined into one quantity", ErrInvalidFarmBuilderSeason)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (s metricStore) addFarmBuilderSeason(tx *gorm.DB, event *Event) error {
	id := event.ID
	if err := s.events(tx).Select("ID", "CreatedAt", "UpdatedAt", "DeletedAt", "PID", "SID", "Env", "Build", "Project",
		"Type", "Level", "RunStep", "Commit", "Platform", "Gold").
		Clauses(clause.OnConflict{DoNothing: true}).Create(event).Error; err != nil {
		return err
	}
	var stored Event
	if err := s.events(tx).Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("p_id = ? AND s_id = ? AND run_step = ?", event.PID, event.SID, *event.RunStep).Take(&stored).Error; err != nil {
		return err
	}
	if stored.ID != id {
		if err := s.items(tx).Where("event_id = ?", stored.ID).Order("slot, item").Find(&stored.Items).Error; err != nil {
			return err
		}
		*event = stored
		return nil
	}
	if len(event.Items) == 0 {
		return nil
	}
	for i := range event.Items {
		event.Items[i].EventID = id
	}
	return s.items(tx).Select("EventID", "Item", "Slot", "Level").Create(&event.Items).Error
}
