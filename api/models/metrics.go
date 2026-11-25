package models

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Only these reviewed, embedded queries can run; the HTTP API never accepts SQL.
//
//go:embed metrics/*
var metricFiles embed.FS

type MetricDefinition struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Display     string                 `json:"display"`
	Settings    map[string]interface{} `json:"settings"`
	Project     string                 `json:"project,omitempty"`
	Section     string                 `json:"section"`
	Row         int                    `json:"row"`
	Col         int                    `json:"col"`
	Width       int                    `json:"width"`
}

type MetricResult struct {
	Columns   []string        `json:"columns"`
	Rows      [][]interface{} `json:"rows"`
	Truncated bool            `json:"truncated"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

func MetricCatalog() []MetricDefinition {
	data, err := metricFiles.ReadFile("metrics/catalog.json")
	if err != nil {
		panic(err)
	}
	var cards []MetricDefinition
	if err := json.Unmarshal(data, &cards); err != nil {
		panic(err)
	}
	return cards
}

func metricDefinition(id int) (MetricDefinition, bool) {
	for _, card := range MetricCatalog() {
		if card.ID == id {
			return card, true
		}
	}
	return MetricDefinition{}, false
}

func metricProject(card MetricDefinition) string {
	if card.Project == "" {
		return cursemarkProject
	}
	return card.Project
}

func metricBody(id int) (string, error) {
	data, err := metricFiles.ReadFile(fmt.Sprintf("metrics/%d.sql", id))
	if err != nil {
		return "", err
	}
	return metricQueryBody(string(data)), nil
}

func withMetricSources(sources, query string) string {
	words := strings.Fields(query)
	if len(words) > 1 && strings.EqualFold(words[0], "WITH") {
		rest := strings.TrimSpace(query[len(words[0]):])
		if strings.EqualFold(words[1], "RECURSIVE") {
			return "WITH RECURSIVE " + sources + ", " + strings.TrimSpace(rest[len(words[1]):])
		}
		return "WITH " + sources + ", " + rest
	}
	return "WITH " + sources + " " + query
}

func gameplaySQL(id int, store metricStore, where []string) (string, error) {
	body, err := metricBody(id)
	if err != nil {
		return "", err
	}
	sources := "events AS (SELECT * FROM " + store.sqlTable("events") + " WHERE " + strings.Join(where, " AND ") + "), " +
		"event_items AS (SELECT * FROM " + store.sqlTable("event_items") + ")"
	return withMetricSources(sources, body), nil
}

func RunMetric(ctx context.Context, id int, project string) (*MetricResult, error) {
	card, ok := metricDefinition(id)
	if !ok || metricProject(card) != project {
		return nil, fmt.Errorf("metric does not belong to project")
	}

	var query string
	if id == 38 || id == 39 {
		table := "crashes"
		if id == 39 {
			table = "feedbacks"
		}
		body, err := metricBody(id)
		if err != nil {
			return nil, err
		}
		shared := metricStore{schema: AnalyticsDB.WithContext(ctx).Migrator().CurrentDatabase()}
		if !validSchemaName(shared.schema) {
			return nil, fmt.Errorf("shared metrics database unavailable")
		}
		query = withMetricSources(table+" AS (SELECT * FROM "+shared.sqlTable(table)+" WHERE project = ?)", body)
	} else {
		store, err := metricsStoreFor(project)
		if err != nil {
			return nil, err
		}
		query, err = gameplaySQL(id, store, []string{store.projectFilter()})
		if err != nil {
			return nil, err
		}
	}
	return runMetricQuery(ctx, query, []interface{}{project})
}

func runMetricQuery(ctx context.Context, query string, args []interface{}) (*MetricResult, error) {
	db, err := AnalyticsDB.DB()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	result := &MetricResult{Columns: columns, Rows: make([][]interface{}, 0), UpdatedAt: time.Now().UTC()}
	for rows.Next() {
		if len(result.Rows) == 1000 {
			result.Truncated = true
			break
		}
		values := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		for i, value := range values {
			if bytes, ok := value.([]byte); ok {
				switch types[i].DatabaseTypeName() {
				case "TINYINT", "SMALLINT", "MEDIUMINT", "INT", "INTEGER", "BIGINT", "DECIMAL", "NUMERIC", "FLOAT", "DOUBLE", "UNSIGNED TINYINT", "UNSIGNED SMALLINT", "UNSIGNED MEDIUMINT", "UNSIGNED INT", "UNSIGNED BIGINT":
					values[i] = json.Number(string(bytes))
				default:
					values[i] = string(bytes)
				}
			}
		}
		result.Rows = append(result.Rows, values)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
