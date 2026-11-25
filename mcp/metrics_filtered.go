package main

import (
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type metricQueryArgs struct {
	ID          int    `json:"id" jsonschema:"Reviewed gameplay metric ID from list_metrics. Report cards 38 and 39 are excluded; use report-read tools."`
	Project     string `json:"project" jsonschema:"Game to query: cursemark or necrovale."`
	CreatedFrom string `json:"created_from,omitempty" jsonschema:"Inclusive stored event created_at bound; RFC3339 with timezone."`
	CreatedTo   string `json:"created_to,omitempty" jsonschema:"Exclusive stored event created_at bound; RFC3339 with timezone."`
	Build       string `json:"build,omitempty" jsonschema:"Exact submitted build identifier."`
	Environment string `json:"environment,omitempty" jsonschema:"Exact event environment."`
	EventType   string `json:"event_type,omitempty" jsonschema:"Exact event type, for example clear_level or player_died. Can change metric cohort semantics."`
	Level       string `json:"level,omitempty" jsonschema:"Exact level identifier. All filters apply before aggregation and baselines."`
}
type crashHealthArgs struct {
	Project string `json:"project" jsonschema:"Exact project ID or all."`
	Days    int    `json:"days,omitempty" jsonschema:"Receipt window: 7, 30 or 90 days. Defaults to 30."`
}

type metricProjectArgs struct {
	Project string `json:"project" jsonschema:"Game to query: cursemark or necrovale."`
}

type metricReadArgs struct {
	ID      int    `json:"id" jsonschema:"Metric ID from list_metrics for this game."`
	Project string `json:"project" jsonschema:"Game to query: cursemark or necrovale."`
}

func metricProjectQuery(project string) (url.Values, error) {
	project = strings.ToLower(strings.TrimSpace(project))
	if project != "cursemark" && project != "necrovale" {
		return nil, fmt.Errorf("choose cursemark or necrovale explicitly")
	}
	return url.Values{"project": {project}}, nil
}

func addMetricAnalysisTools(s *mcp.Server, b *backend) {
	mcp.AddTool(s, &mcp.Tool{Name: "list_metrics", Description: "List the reviewed metric definitions for Cursemark or Necrovale.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, a metricProjectArgs) (*mcp.CallToolResult, any, error) {
		q, err := metricProjectQuery(a.Project)
		if err != nil {
			return nil, nil, err
		}
		return b.call(ctx, http.MethodGet, "/metrics?"+q.Encode())
	})
	mcp.AddTool(s, &mcp.Tool{Name: "get_metric", Description: "Run a catalog metric for the selected game.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, a metricReadArgs) (*mcp.CallToolResult, any, error) {
		q, err := metricProjectQuery(a.Project)
		if err != nil || a.ID <= 0 {
			return nil, nil, fmt.Errorf("choose a positive metric ID and explicit cursemark or necrovale project")
		}
		return b.call(ctx, http.MethodGet, "/metrics/"+strconv.Itoa(a.ID)+"?"+q.Encode())
	})
	mcp.AddTool(s, &mcp.Tool{Name: "query_metric", Description: "Read a reviewed Cursemark or Necrovale gameplay aggregate. Filters scope source events before aggregation and baselines. Returns applied filters, cached computation time, row truncation and cohort details.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, a metricQueryArgs) (*mcp.CallToolResult, any, error) {
		q, err := metricProjectQuery(a.Project)
		if err != nil || a.ID <= 0 {
			return nil, nil, fmt.Errorf("choose a positive gameplay metric ID and explicit cursemark or necrovale project")
		}
		for _, f := range []struct{ name, value string }{{"created_from", a.CreatedFrom}, {"created_to", a.CreatedTo}, {"build", a.Build}, {"environment", a.Environment}, {"event_type", a.EventType}, {"level", a.Level}} {
			if f.value != "" {
				if len(f.value) > 128 || strings.ContainsAny(f.value, "\x00\r\n") {
					return nil, nil, fmt.Errorf("invalid metric filter")
				}
				if f.name == "created_from" || f.name == "created_to" {
					if _, e := time.Parse(time.RFC3339Nano, f.value); e != nil {
						return nil, nil, fmt.Errorf("timestamp requires RFC3339 timezone")
					}
				}
				q.Set(f.name, f.value)
			}
		}
		return b.call(ctx, http.MethodGet, "/metric-query/"+strconv.Itoa(a.ID)+"?"+q.Encode())
	})
	mcp.AddTool(s, &mcp.Tool{Name: "get_crash_health", Description: "Read scoped crash receipt/build/returned-incident aggregates. Detailed receipt events and retained legacy totals remain distinct. Reporting IDs are client PIDs, not verified people or an exposure denominator. No raw stacks or identifiers; not a complete group membership index. Does not change report status.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, a crashHealthArgs) (*mcp.CallToolResult, any, error) {
		if a.Project == "" || len(a.Project) > 64 || strings.ContainsAny(a.Project, "\x00\r\n") {
			return nil, nil, fmt.Errorf("invalid project")
		}
		if a.Days == 0 {
			a.Days = 30
		}
		if a.Days != 7 && a.Days != 30 && a.Days != 90 {
			return nil, nil, fmt.Errorf("days must be 7, 30 or 90")
		}
		q := url.Values{"project": {a.Project}, "days": {strconv.Itoa(a.Days)}}
		return b.call(ctx, http.MethodGet, "/crash-health?"+q.Encode())
	})
}
