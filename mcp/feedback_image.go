package main

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Matches the stored screenshot limit, not the smaller JSON response limit.
const maxFeedbackImageResponse = 5 << 20

type feedbackImageArgs struct {
	ID      int    `json:"id" jsonschema:"Positive open feedback item ID."`
	Project string `json:"project" jsonschema:"Exact project from get_feedback, for example cursemark."`
}

func (b *backend) feedbackImage(ctx context.Context, _ *mcp.CallToolRequest, args feedbackImageArgs) (*mcp.CallToolResult, any, error) {
	if args.ID <= 0 || args.Project == "" {
		return nil, nil, fmt.Errorf("a positive feedback ID and its project are required")
	}
	path := "/internal/mcp/feedback/" + strconv.Itoa(args.ID) + "/image" + reportQuery(&args.Project)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.base+path, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+b.token)
	req.Header.Set("Accept", "image/jpeg")
	res, err := b.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("admin API unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "No screenshot is available for this feedback ID and project."}}}, nil, nil
	}
	if res.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("admin API returned HTTP %d", res.StatusCode)
	}
	kind, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if err != nil || kind != "image/jpeg" || res.ContentLength > maxFeedbackImageResponse {
		return nil, nil, fmt.Errorf("invalid or oversized feedback image")
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxFeedbackImageResponse+1))
	if err != nil || len(raw) > maxFeedbackImageResponse {
		return nil, nil, fmt.Errorf("feedback image could not be read within its size limit")
	}
	if _, err = jpeg.DecodeConfig(bytes.NewReader(raw)); err != nil {
		return nil, nil, fmt.Errorf("invalid feedback JPEG")
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.ImageContent{Data: raw, MIMEType: "image/jpeg"}}}, nil, nil
}
