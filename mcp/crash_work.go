package main

import (
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/url"
	"strconv"
)

type crashWorkArgs struct {
	ID      int    `json:"id" jsonschema:"Any evidence report ID in the group; all members address one canonical investigation"`
	Project string `json:"project"`
	AfterID int    `json:"after_id,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}
type crashClaimArgs struct {
	ID         int    `json:"id"`
	Project    string `json:"project"`
	RequestKey string `json:"request_key" jsonschema:"Unique stable key for this worker run; retry exactly; never choose a fresh key just to evade another claim"`
	ClaimToken string `json:"claim_token,omitempty" jsonschema:"Only to renew an active claim; omit for acquire or exact acquire retry"`
}
type crashAppendArgs struct {
	ID           int    `json:"id"`
	Project      string `json:"project"`
	RequestKey   string `json:"request_key"`
	Note         string `json:"note"`
	State        string `json:"state,omitempty"`
	ReleaseState string `json:"release_state,omitempty"`
	ClaimToken   string `json:"claim_token"`
	Complete     bool   `json:"complete,omitempty" jsonschema:"True after saving the investigation outcome; acknowledges only evidence captured when the claim was granted"`
}

func crashWorkPath(id int, project string) (string, error) {
	if id < 1 {
		return "", fmt.Errorf("positive evidence anchor id required")
	}
	return "/crash-investigation/" + strconv.Itoa(id) + "?project=" + url.QueryEscape(project), nil
}
func addCrashWorkTools(s *mcp.Server, b *backend) {
	mcp.AddTool(s, &mcp.Tool{Name: "get_crash_investigation", Description: "Read ONE canonical group investigation, status, new-evidence/returned flags and chronological notes, including legacy per-report history. Any member ID maps to the same unit key. Access Violations share one investigation per project while retaining real incident identities. The complete report_ids index includes resolved history; get_crash reads any member as evidence. Reports are evidence, never separate investigations. Follow note pagination. Never close reports.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, a crashWorkArgs) (*mcp.CallToolResult, any, error) {
		p, e := crashWorkPath(a.ID, a.Project)
		if e != nil {
			return nil, nil, e
		}
		if a.AfterID < 0 || a.Limit < 0 || a.Limit > 50 {
			return nil, nil, fmt.Errorf("invalid cursor/limit")
		}
		if a.Limit > 0 {
			p += "&limit=" + strconv.Itoa(a.Limit)
		}
		p += "&after_id=" + strconv.Itoa(a.AfterID)
		return b.call(ctx, http.MethodGet, p)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "claim_crash_investigation", Description: "REQUIRED before investigating a crash group. Read its group log, then acquire one durable 30-minute worker lease. Do work only when acquired=true. Already claimed/reviewed/closed/completed means skip; do not start per-report work. Stable request_key gives safe retry; claim_token renews an unexpired lease. New evidence after a finished run is reviewed in the SAME investigation. Do not expose the returned token in reports.", Annotations: &mcp.ToolAnnotations{IdempotentHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, a crashClaimArgs) (*mcp.CallToolResult, any, error) {
		_, e := crashWorkPath(a.ID, a.Project)
		if e != nil {
			return nil, nil, e
		}
		return b.callJSON(ctx, http.MethodPost, "/crash-investigation/"+strconv.Itoa(a.ID)+"/claim?project="+url.QueryEscape(a.Project), map[string]any{"request_key": a.RequestKey, "claim_token": a.ClaimToken})
	})
	mcp.AddTool(s, &mcp.Tool{Name: "append_crash_investigation_note", Description: "Append concise canonical GROUP investigation context using the active claim token, never per-report notes. Set complete=true and an outcome state when finished; it releases the lease and acknowledges only the claimed evidence snapshot. Keep request_key/payload identical on uncertain retry, even after lease completion. New arrivals remain pending. A lifecycle/manual-note conflict requires rereading. Local is unreleased. No resolution or ticket changes.", Annotations: &mcp.ToolAnnotations{IdempotentHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, a crashAppendArgs) (*mcp.CallToolResult, any, error) {
		_, e := crashWorkPath(a.ID, a.Project)
		if e != nil {
			return nil, nil, e
		}
		if a.ClaimToken == "" {
			return nil, nil, fmt.Errorf("active group claim token required")
		}
		return b.callJSON(ctx, http.MethodPost, "/crash-investigation/"+strconv.Itoa(a.ID)+"/notes?project="+url.QueryEscape(a.Project), map[string]any{"request_key": a.RequestKey, "note": a.Note, "state": a.State, "release_state": a.ReleaseState, "claim_token": a.ClaimToken, "complete": a.Complete})
	})
}
