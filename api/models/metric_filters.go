package models

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Reviewed aggregates only. Raw crash/feedback catalog cards use dedicated reads.
func GameplayMetricAllowed(id int) bool {
	_, ok := metricDefinition(id)
	return ok && id != 38 && id != 39
}

func GameplayMetricForProject(id int, project string) bool {
	card, ok := metricDefinition(id)
	return ok && id != 38 && id != 39 && metricProject(card) == project
}

type MetricFilters struct {
	Project     string `json:"project"`
	CreatedFrom string `json:"created_from,omitempty"`
	CreatedTo   string `json:"created_to,omitempty"`
	Build       string `json:"build,omitempty"`
	Environment string `json:"environment,omitempty"`
	EventType   string `json:"event_type,omitempty"`
	Level       string `json:"level,omitempty"`
}

func (f MetricFilters) Normalize() (MetricFilters, error) {
	f.Project = strings.ToLower(strings.TrimSpace(f.Project))
	if f.Project != cursemarkProject && f.Project != necrovaleProject && f.Project != FarmBuilderProject {
		return f, fmt.Errorf("choose cursemark, necrovale, or farm-builder explicitly")
	}
	for _, p := range []*string{&f.Build, &f.Environment, &f.EventType, &f.Level} {
		*p = strings.TrimSpace(*p)
		if len(*p) > 128 {
			return f, fmt.Errorf("filter value exceeds 128 bytes")
		}
		for _, r := range *p {
			if unicode.IsControl(r) {
				return f, fmt.Errorf("filter contains control characters")
			}
		}
	}
	var bounds [2]time.Time
	for i, p := range []*string{&f.CreatedFrom, &f.CreatedTo} {
		if *p == "" {
			continue
		}
		if len(*p) > 64 {
			return f, fmt.Errorf("invalid event timestamp")
		}
		v, e := time.Parse(time.RFC3339Nano, *p)
		if e != nil {
			return f, fmt.Errorf("event timestamps require RFC3339 with a timezone")
		}
		v = v.UTC()
		*p = v.Format(time.RFC3339Nano)
		bounds[i] = v
	}
	if !bounds[0].IsZero() && !bounds[1].IsZero() && !bounds[0].Before(bounds[1]) {
		return f, fmt.Errorf("created_from must be before created_to")
	}
	return f, nil
}

// Leading comments are reviewed source text, not caller-supplied SQL. Removing
// them before WITH composition also fixes the commented catalog query 132.
func metricQueryBody(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(s, "\ufeff"))
	for {
		if strings.HasPrefix(s, "--") {
			if at := strings.IndexByte(s, '\n'); at >= 0 {
				s = strings.TrimSpace(s[at+1:])
				continue
			}
			return ""
		}
		if strings.HasPrefix(s, "/*") {
			if at := strings.Index(s, "*/"); at >= 0 {
				s = strings.TrimSpace(s[at+2:])
				continue
			}
		}
		return s
	}
}
func filteredMetricSQL(id int, f MetricFilters, schema string) (string, []interface{}, error) {
	if !GameplayMetricForProject(id, f.Project) {
		return "", nil, fmt.Errorf("metric is not a gameplay aggregate for the selected project")
	}
	if !validSchemaName(schema) {
		return "", nil, fmt.Errorf("invalid metrics schema")
	}
	store := metricStore{schema: schema, project: f.Project}
	where := []string{"deleted_at IS NULL", store.projectFilter()}
	args := []interface{}{f.Project}
	for _, bound := range []struct{ value, op string }{{f.CreatedFrom, ">="}, {f.CreatedTo, "<"}} {
		if bound.value != "" {
			v, _ := time.Parse(time.RFC3339Nano, bound.value)
			where = append(where, "created_at "+bound.op+" ?")
			args = append(args, v)
		}
	}
	for _, filter := range []struct{ column, value string }{{"build", f.Build}, {"env", f.Environment}, {"type", f.EventType}, {"level", f.Level}} {
		if filter.value != "" {
			where = append(where, filter.column+" = ?")
			args = append(args, filter.value)
		}
	}
	query, err := gameplaySQL(id, store, where)
	return query, args, err
}

func RunFilteredMetric(ctx context.Context, id int, f MetricFilters) (*MetricResult, error) {
	f, err := f.Normalize()
	if err != nil {
		return nil, err
	}
	store, err := metricsStoreFor(f.Project)
	if err != nil {
		return nil, err
	}
	query, args, err := filteredMetricSQL(id, f, store.schema)
	if err != nil {
		return nil, err
	}
	result, e := runMetricQuery(ctx, query, args)
	if e != nil {
		return nil, e
	}
	for _, column := range result.Columns {
		switch strings.ToLower(column) {
		case "p_id", "pid", "s_id", "sid", "message", "stack":
			return nil, fmt.Errorf("raw identifiers or report text are not allowed in gameplay aggregates")
		}
	}
	return result, nil
}
