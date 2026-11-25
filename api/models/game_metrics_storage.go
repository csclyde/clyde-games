package models

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const necrovaleProject = "necrovale"

type metricStore struct{ schema, project string }

var gameMetricStores []metricStore
var ErrMetricsStoreUnavailable = errors.New("metrics store unavailable")

func quoteIdentifier(name string) string { return "`" + name + "`" }

func (s metricStore) table(name string) string {
	if s.schema == "" {
		return name
	}
	return s.schema + "." + name
}

func (s metricStore) sqlTable(name string) string {
	return quoteIdentifier(s.schema) + "." + quoteIdentifier(name)
}

func validSchemaName(name string) bool {
	return name != "" && len(name) <= 64 && strings.IndexFunc(name, func(r rune) bool {
		return !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) == -1
}

func newGameMetricStores(cursemark, necrovale, shared string) ([]metricStore, error) {
	if !validSchemaName(cursemark) || !validSchemaName(necrovale) || !validSchemaName(shared) ||
		strings.EqualFold(cursemark, necrovale) || strings.EqualFold(shared, cursemark) || strings.EqualFold(shared, necrovale) {
		return nil, fmt.Errorf("metrics stores require three distinct MySQL databases")
	}
	return []metricStore{{cursemark, cursemarkProject}, {necrovale, necrovaleProject}}, nil
}

// ConfigureGameMetrics runs before serving HTTP. An incomplete split fails startup.
func ConfigureGameMetrics(cursemark, necrovale string) (func(), error) {
	shared := AnalyticsDB.Migrator().CurrentDatabase()
	stores, err := newGameMetricStores(cursemark, necrovale, shared)
	if err != nil {
		return nil, err
	}
	var schemas struct{ Cursemark, Necrovale string }
	result := AnalyticsDB.Raw("SELECT cursemark, necrovale FROM game_metrics_split WHERE id = 1").Scan(&schemas)
	if result.Error != nil || result.RowsAffected != 1 || schemas.Cursemark != cursemark || schemas.Necrovale != necrovale {
		return nil, fmt.Errorf("game metrics migration has not activated the configured databases")
	}
	for _, store := range stores {
		for _, table := range []string{"events", "event_items"} {
			if err := AnalyticsDB.Exec("SELECT 1 FROM " + store.sqlTable(table) + " LIMIT 0").Error; err != nil {
				return nil, fmt.Errorf("metrics storage unavailable for %s: %w", store.project, err)
			}
		}
	}
	previous := gameMetricStores
	gameMetricStores = stores
	return func() { gameMetricStores = previous }, nil
}

func metricsStoreFor(project string) (metricStore, error) {
	for _, store := range gameMetricStores {
		if store.project == project {
			return store, nil
		}
	}
	return metricStore{}, fmt.Errorf("%w for %s", ErrMetricsStoreUnavailable, project)
}

func eventStoreFor(project string) metricStore {
	if project == "" {
		project = cursemarkProject // Project-less game clients predate Necrovale collection.
	}
	for _, store := range gameMetricStores {
		if store.project == project {
			return store
		}
	}
	return metricStore{} // Existing collection for other projects.
}

func allEventStores() []metricStore {
	return append([]metricStore{{}}, gameMetricStores...)
}

func (s metricStore) events(db *gorm.DB) *gorm.DB {
	return db.Model(&Event{}).Table(s.table("events"))
}

func (s metricStore) items(db *gorm.DB) *gorm.DB {
	return db.Model(&EventItem{}).Table(s.table("event_items"))
}

func (s metricStore) projectFilter() string {
	if s.project == cursemarkProject {
		return "(project = ? OR project IS NULL OR TRIM(project) = '')"
	}
	return "project = ?"
}

// A shared sequence keeps event IDs unique for admin history and a lossless
// return to the legacy schema. Allocate on a pinned, autocommit connection so
// the sequence lock is released before the event and item transaction.
func nextGameEventID() (uint, error) {
	var id uint
	err := AnalyticsDB.Connection(func(db *gorm.DB) error {
		db = db.Session(&gorm.Session{NewDB: true})
		result := db.Exec("UPDATE game_metrics_split SET next_id = LAST_INSERT_ID(next_id + 1) WHERE id = 1")
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("game metrics sequence is unavailable")
		}
		return db.Raw("SELECT LAST_INSERT_ID()").Scan(&id).Error
	})
	return id, err
}
