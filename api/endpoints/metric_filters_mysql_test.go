package endpoints

import (
	"api.clyde.games/models"
	"context"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"io"
	"strconv"
	"testing"
	"time"
)

func TestFilteredMetricsMySQLCohortAndNestedBaseline(t *testing.T) {
	db := syncTestDB(t)
	if e := db.AutoMigrate(&models.Event{}, &models.EventItem{}); e != nil {
		t.Fatal(e)
	}
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	old := at.AddDate(0, -1, 0)
	base := func(project, build, env, kind, level string, curse int, when time.Time) models.Event {
		return models.Event{Model: gorm.Model{CreatedAt: when, UpdatedAt: when}, Project: project, Build: build, Env: env, Type: kind, Level: level, Cursemark: curse, PID: "pid", SID: fmt.Sprintf("run-%d", curse)}
	}
	rows := []models.Event{base("cursemark", "A", "production", "clear_level", "LevelA", 1, at), base("cursemark", "A", "production", "player_died", "LevelA", 2, at), base("cursemark", "B", "production", "clear_level", "LevelA", 3, at), base("cursemark", "A", "testing", "clear_level", "LevelA", 4, at), base("necrovale", "A", "production", "clear_level", "LevelA", 0, at), base("cursemark", "A", "production", "clear_level", "LevelA", 5, old), base("cursemark", "A", "production", "clear_level", "LevelB", 6, at), base("cursemark", "A", "production", "clear_level", "LevelA", 7, at)}
	rows[7].DeletedAt = gorm.DeletedAt{Time: at, Valid: true}
	if e := db.Create(&rows).Error; e != nil {
		t.Fatal(e)
	}
	shared := db.Migrator().CurrentDatabase()
	cursemark, necrovale := shared+"_cm", shared+"_nv"
	t.Cleanup(func() {
		db.Exec("DROP DATABASE IF EXISTS " + cursemark)
		db.Exec("DROP DATABASE IF EXISTS " + necrovale)
	})
	if err := models.MigrateGameMetrics(db, cursemark, necrovale, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	restore, err := models.ConfigureGameMetrics(cursemark, necrovale)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	filters := models.MetricFilters{Project: "cursemark", CreatedFrom: "2026-11-01T00:00:00Z", CreatedTo: "2026-11-02T00:00:00Z", Build: "A", Environment: "production", Level: "LevelA"}
	r, e := models.RunFilteredMetric(context.Background(), 104, filters)
	if e != nil || len(r.Rows) != 2 {
		t.Fatal(r, e)
	}
	for _, row := range r.Rows {
		if fmt.Sprint(row[0]) != "1" && fmt.Sprint(row[0]) != "2" {
			t.Fatal("cohort leak", row)
		}
	}
	for i := 0; i < 60; i++ {
		for _, b := range []string{"A", "B"} {
			depth := 10
			item := "Sword"
			if b == "B" {
				depth = 100
				item = "Staff"
			}
			event := base("cursemark", b, "production", "clear_level", "LevelA", depth, at)
			if e := db.Table(cursemark+".events").Omit("hopestones", "gold", "lifetime_clears", "lifetime_deaths", "clear_history_complete", "death_history_complete").Create(&event).Error; e != nil {
				t.Fatal(e)
			}
			if e := db.Table(cursemark + ".event_items").Create(&models.EventItem{EventID: event.ID, Item: item, Slot: "weapon", Level: 1}).Error; e != nil {
				t.Fatal(e)
			}
		}
	}
	var before, after []models.Event
	db.Table(cursemark + ".events").Unscoped().Order("id").Find(&before)
	stamp, _ := json.Marshal(before)
	for _, card := range models.MetricCatalog() {
		if !models.GameplayMetricForProject(card.ID, "cursemark") {
			continue
		}
		if _, e := models.RunFilteredMetric(context.Background(), card.ID, filters); e != nil {
			t.Fatalf("metric %d syntax/cohort: %v", card.ID, e)
		}
	}
	r, e = models.RunFilteredMetric(context.Background(), 102, filters)
	if e != nil || len(r.Rows) != 1 {
		t.Fatal(r, e)
	}
	col := map[string]int{}
	for i, name := range r.Columns {
		col[name] = i
	}
	value := func(name string) float64 {
		v, e := strconv.ParseFloat(fmt.Sprint(r.Rows[0][col[name]]), 64)
		if e != nil {
			t.Fatal(name, e)
		}
		return v
	}
	if r.Rows[0][col["item"]] != "Sword" || value("avg_cursemark") != 10 || value("global_avg_cursemark") != 10 || value("above_global_avg") != 0 {
		t.Fatal("baseline escaped filtered cohort", r)
	}
	db.Table(cursemark + ".events").Unscoped().Order("id").Find(&after)
	next, _ := json.Marshal(after)
	if string(stamp) != string(next) {
		t.Fatal("metric read mutated source events")
	}
}
