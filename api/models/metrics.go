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
//go:embed metrics/*
var metricFiles embed.FS

type MetricDefinition struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Display     string                 `json:"display"`
	Settings    map[string]interface{} `json:"settings"`
	Project     string `json:"project,omitempty"`
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

func metricSQL(id int, project string) (string, []interface{}, error) {
	var definition *MetricDefinition
	for _, card := range MetricCatalog() {
		if card.ID == id { definition = &card; break }
	}
	if definition == nil { return "", nil, fmt.Errorf("unknown metric") }
	data, err := metricFiles.ReadFile(fmt.Sprintf("metrics/%d.sql", id))
	if err != nil { return "", nil, err }
	query := metricQueryBody(string(data))
	all := project == "" || project == "all"
	if definition.Project != "" {
		if !all && !strings.EqualFold(project, definition.Project) { return "", nil, fmt.Errorf("metric does not belong to project") }
		project, all = definition.Project, false
	}
	table := "events"
	if id == 38 { table = "crashes" }
	if id == 39 { table = "feedbacks" }
	var prefix string
	var args []interface{}
	if all {
		if table != "events" { return query, nil, nil }
		prefix = "WITH events AS (SELECT * FROM analytics.events WHERE project IS NULL OR LOWER(project) <> 'necrovale') "
	} else {
		args = []interface{}{project}
		prefix = fmt.Sprintf("WITH %s AS (SELECT * FROM analytics.%s WHERE project = ?) ", table, table)
		if table == "events" && definition.Project == "" {
			prefix = "WITH events AS (SELECT * FROM analytics.events WHERE project = ? AND LOWER(project) <> 'necrovale') "
		}
	}
	if strings.HasPrefix(strings.ToUpper(query), "WITH RECURSIVE") {
		query = "WITH RECURSIVE " + strings.TrimPrefix(prefix, "WITH ") + ", " + strings.TrimSpace(query[len("WITH RECURSIVE"):])
	} else if strings.HasPrefix(strings.ToUpper(query), "WITH ") {
		query = prefix + ", " + query[5:]
	} else { query = prefix + query }
	return query, args, nil
}

func RunMetric(ctx context.Context, id int, project string) (*MetricResult, error) {
	query, args, err := metricSQL(id, project)
	if err != nil {
		return nil, err
	}
	return runMetricQuery(ctx, query, args)
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
