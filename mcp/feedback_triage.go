package main

import (
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"strconv"
)

type feedbackTriageArgs struct {
	ID         int     `json:"id"`
	State      string  `json:"state" jsonschema:"pending, reviewed, or noise; independent of resolution"`
	Note       string  `json:"note" jsonschema:"Brief reason at most 2000 characters"`
	Translated *string `json:"translated,omitempty" jsonschema:"Optional translation at most 30000 characters, preserve original message"`
}

func addFeedbackTriageTool(s *mcp.Server, b *backend) {
	mcp.AddTool(s, &mcp.Tool{Name: "triage_feedback", Description: "Save reversible pending/reviewed/noise triage and optional translation after reading the feedback. Preserves original text and resolved status. Noise means reviewed non-actionable content; unknown items stay pending. Does not resolve or delete anything.", Annotations: &mcp.ToolAnnotations{IdempotentHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, args feedbackTriageArgs) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 || (args.State != "pending" && args.State != "reviewed" && args.State != "noise") || len([]rune(args.Note)) > 2000 || (args.Translated != nil && len([]rune(*args.Translated)) > 30000) {
			return nil, nil, fmt.Errorf("invalid feedback triage")
		}
		return b.callJSON(ctx, http.MethodPost, "/feedback/"+strconv.Itoa(args.ID)+"/triage", args)
	})
}
