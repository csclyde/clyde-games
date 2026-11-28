package endpoints

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func TestMCPFeedbackImageMySQLLifecycle(t *testing.T) {
	db := syncTestDB(t)
	if err := db.AutoMigrate(&models.Savegame{}, &models.FeedbackImage{}); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("m", 40)
	t.Setenv("MCP_UPSTREAM_TOKEN", token)
	r := gin.New()
	RegisterMCPRoutes(r)
	r.POST("/feedback", AddFeedback)
	const metadata = `{"Project":"cursemark","PID":"mcp-fixture","Message":"image evidence"}`
	raw, ct := multipartImage(t, metadata, imageFixture(t, 6, 4))
	post := func(ct string, raw []byte) string {
		t.Helper()
		w := testImageRoute(r, "POST", "/feedback", "", ct, raw)
		var f models.Feedback
		if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &f) != nil {
			t.Fatalf("fixture creation failed: HTTP %d", w.Code)
		}
		return strconv.FormatUint(uint64(f.ID), 10)
	}
	id := post(ct, raw)
	emptyID := post("application/json", []byte(metadata))
	read := func(method, path, auth string, want int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/internal/mcp/feedback/"+path, nil)
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s HTTP %d, want %d", path, w.Code, want)
		}
		return w
	}
	read(http.MethodGet, id+"/image?project=cursemark", "", 401)
	read(http.MethodGet, id+"/image", token, 400)
	read(http.MethodGet, id+"/image?project=all", token, 400)
	read(http.MethodGet, id+"/image?project=necrovale", token, 404)
	read(http.MethodGet, emptyID+"/image?project=cursemark", token, 404)
	w := read(http.MethodGet, id+"/image?project=cursemark", token, 200)
	if w.Header().Get("Content-Type") != "image/jpeg" || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("incorrect image headers")
	}
	if !bytes.Contains(read(http.MethodGet, id+"?project=cursemark", token, 200).Body.Bytes(), []byte(`"has_image":true`)) {
		t.Fatal("screenshot not advertised")
	}
	read(http.MethodPost, id+"/resolve", token, 200)
	read(http.MethodGet, id+"/image?project=cursemark", token, 404)
	var n int64
	if err := db.Model(&models.FeedbackImage{}).Where("feedback_id = ?", id).Count(&n).Error; err != nil || n != 0 {
		t.Fatal("closed screenshot persisted", n, err)
	}
}
