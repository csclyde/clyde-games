package main

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestFeedbackImageToolThroughMCP(t *testing.T) {
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 6, 4)), nil); err != nil {
		t.Fatal(err)
	}
	token := "123456789012345678901234567890123456"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/internal/mcp/feedback/7/image" || r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("unexpected upstream request")
			w.WriteHeader(401)
			return
		}
		if r.URL.Query().Get("project") != "cursemark" {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(jpg.Bytes())
	}))
	defer upstream.Close()
	b, err := newBackend(upstream.URL, token)
	if err != nil {
		t.Fatal(err)
	}
	ct, st := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ss, err := makeServer(b).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "fixture", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	for _, p := range []string{"cursemark", "necrovale"} {
		result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_feedback_image", Arguments: map[string]any{"id": 7, "project": p}})
		if err != nil || result.IsError || len(result.Content) != 1 {
			t.Fatalf("unexpected tool result: %v %v", result, err)
		}
		if p == "cursemark" {
			im, ok := result.Content[0].(*mcp.ImageContent)
			if !ok || im.MIMEType != "image/jpeg" || !bytes.Equal(im.Data, jpg.Bytes()) {
				t.Fatal("did not return actual JPEG")
			}
		} else if _, ok := result.Content[0].(*mcp.TextContent); !ok {
			t.Fatal("missing no-image result")
		}
	}
}
