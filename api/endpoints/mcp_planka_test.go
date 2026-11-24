package endpoints

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestMCPPlankaLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var mu sync.Mutex
	cards := []mcpPlankaCard{}
	comments := []map[string]any{}
	creates, moves, notes := 0, 0, 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		if r.URL.Path == "/api/access-tokens" {
			reply(map[string]any{"item": "test-token"})
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing auth")
			w.WriteHeader(401)
			return
		}
		switch {
		case r.URL.Path == "/api/projects":
			reply(map[string]any{"items": []any{map[string]any{"id": "1", "name": "Game"}}, "included": map[string]any{"boards": []any{map[string]any{"id": "2", "projectId": "1", "name": "Active"}, map[string]any{"id": "3", "projectId": "1", "name": "Completed"}}}})
		case r.URL.Path == "/api/boards/2":
			reply(map[string]any{"item": map[string]any{"id": "2"}, "included": map[string]any{"lists": []any{map[string]any{"id": "4", "name": "Open"}}, "cards": cards}})
		case r.URL.Path == "/api/boards/3":
			reply(map[string]any{"item": map[string]any{"id": "3"}, "included": map[string]any{"lists": []any{map[string]any{"id": "5", "name": "Done"}}, "cards": []any{}}})
		case r.URL.Path == "/api/lists/4/cards" && r.Method == "POST":
			creates++
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			card := mcpPlankaCard{ID: "6", Name: body["name"].(string), Description: body["description"].(string), BoardID: "2", ListID: "4"}
			cards = append(cards, card)
			reply(map[string]any{"item": card})
		case r.URL.Path == "/api/cards/6/actions":
			if r.URL.Query().Get("beforeId") == "" {
				reply(map[string]any{"items": comments})
			} else {
				reply(map[string]any{"items": []any{}})
			}
		case r.URL.Path == "/api/cards/6/comment-actions" && r.Method == "POST":
			notes++
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			comments = append(comments, map[string]any{"id": "7", "data": map[string]any{"text": body["text"]}})
			reply(map[string]any{"item": map[string]any{"id": "7"}})
		case r.URL.Path == "/api/cards/6" && r.Method == "PATCH":
			moves++
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, ok := body["description"]; ok {
				t.Error("move changed description")
			}
			reply(map[string]any{"item": map[string]any{"id": "6", "boardId": body["boardId"], "listId": body["listId"]}})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	t.Setenv("PLANKA_URL", upstream.URL)
	t.Setenv("PLANKA_USERNAME", "test")
	t.Setenv("PLANKA_PASSWORD", "test")
	t.Setenv("MCP_UPSTREAM_TOKEN", strings.Repeat("a", 32))
	router := gin.New()
	RegisterMCPRoutes(router)
	call := func(method, path, body string, expected int) {
		t.Helper()
		r := httptest.NewRequest(method, "/internal/mcp/planka"+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 32))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != expected {
			t.Fatalf("%s: got %d: %s", path, w.Code, w.Body.String())
		}
	}
	call("GET", "/boards", "", 200)
	call("GET", "/tickets", "", 200)
	body := `{"board_id":"2","list_id":"4","name":"Bug","description":"Evidence","source_key":"feedback:123"}`
	call("POST", "/tickets", body, 200)
	call("POST", "/tickets", body, 200)
	call("POST", "/tickets", strings.ReplaceAll(body, `"4"`, `"5"`), 200) // existing source returned even when moved destination differs
	call("POST", "/tickets", strings.ReplaceAll(strings.ReplaceAll(body, `"4"`, `"999"`), "123", "124"), 400)
	call("POST", "/tickets", strings.ReplaceAll(body, `"2"`, `"../2"`), 400)
	note := `{"id":"6","note":"Evidence update","note_key":"analysis:1"}`
	call("POST", "/note", note, 200)
	call("POST", "/note", note, 200)
	move := `{"id":"6","board_id":"3","list_id":"5","owner_confirmed":false}`
	call("POST", "/move", move, 403)
	call("POST", "/move", strings.ReplaceAll(move, "false", "true"), 200)
	call("POST", "/move", strings.ReplaceAll(strings.ReplaceAll(move, "false", "true"), `"5"`, `"4"`), 400)
	if creates != 1 || moves != 1 || notes != 1 {
		t.Fatalf("unexpected mutations create=%d move=%d note=%d", creates, moves, notes)
	}
	if cards[0].Description != "Evidence\n\n[clyde-source:feedback:123]" {
		t.Fatal("ticket content overwritten")
	}
}
func TestPlankaMutationsRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("MCP_UPSTREAM_TOKEN", strings.Repeat("a", 32))
	r := gin.New()
	RegisterMCPRoutes(r)
	for _, p := range []string{"tickets", "note", "move"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/internal/mcp/planka/"+p, strings.NewReader(`{}`)))
		if w.Code != 401 {
			t.Fatal(p, w.Code)
		}
	}
}

func TestMCPPlankaMoveRejectsUnverifiedResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ name, response string }{
		{"missing item", `{}`},
		{"null item", `{"item":null}`},
		{"missing destination", `{"item":{"id":"6"}}`},
		{"wrong ticket", `{"item":{"id":"7","boardId":"2","listId":"4"}}`},
		{"wrong board", `{"item":{"id":"6","boardId":"3","listId":"4"}}`},
		{"wrong list", `{"item":{"id":"6","boardId":"2","listId":"5"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patches := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/access-tokens":
					w.Write([]byte(`{"item":"test-token"}`))
				case "/api/projects":
					w.Write([]byte(`{"items":[{"id":"1"}],"included":{"boards":[{"id":"2","projectId":"1"}]}}`))
				case "/api/boards/2":
					w.Write([]byte(`{"item":{"id":"2"},"included":{"lists":[{"id":"4"}],"cards":[]}}`))
				case "/api/cards/6":
					if r.Method != "PATCH" {
						t.Errorf("unexpected method %s", r.Method)
					}
					patches++
					w.Write([]byte(tc.response))
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer upstream.Close()
			t.Setenv("PLANKA_URL", upstream.URL)
			t.Setenv("PLANKA_USERNAME", "test")
			t.Setenv("PLANKA_PASSWORD", "test")
			t.Setenv("MCP_UPSTREAM_TOKEN", strings.Repeat("a", 32))
			router := gin.New()
			RegisterMCPRoutes(router)
			r := httptest.NewRequest("POST", "/internal/mcp/planka/move", strings.NewReader(`{"id":"6","board_id":"2","list_id":"4","owner_confirmed":true}`))
			r.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 32))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), `"outcome_uncertain":true`) || strings.Contains(w.Body.String(), `"moved":true`) {
				t.Fatalf("unverified move reported incorrectly: %d %s", w.Code, w.Body.String())
			}
			if patches != 1 {
				t.Fatalf("must not retry uncertain move: %d writes", patches)
			}
		})
	}
}
