package endpoints

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"api.clyde.games/models"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func TestReportNoteAdminAuthentication(t *testing.T) {
	r := gin.New()
	RegisterReportNoteRoutes(r)
	for _, route := range r.Routes() {
		p := strings.ReplaceAll(strings.ReplaceAll(route.Path, ":kind", "feedback"), ":id", "1")
		req := httptest.NewRequest(route.Method, p+"?project=cursemark", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("%s %s %d", route.Method, p, w.Code)
		}
	}
}
func TestReportNoteAppendLocksIndividualWithoutResolutionWrites(t *testing.T) {
	r, m := mcpMockDB(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT .* FROM `feedbacks` WHERE deleted_at IS NULL AND id = \\? LIMIT 1 FOR UPDATE").WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"id", "project", "resolved"}).AddRow(7, "cursemark", true))
	m.ExpectQuery("SELECT .* FROM `report_notes` WHERE kind = \\? AND report_id = \\? AND request_key = \\? LIMIT 1").WithArgs("feedback", 7, "test-lock").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectExec("INSERT INTO `report_notes`").WillReturnResult(sqlmock.NewResult(42, 1))
	m.ExpectCommit()
	w := noteRequest(r, "POST", "report-notes/feedback/7?project=cursemark", `{"request_key":"test-lock","note":"Tested locally; no released build verified.","state":"fixed","release_state":"local"}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"actor":"mcp"`) || !strings.Contains(w.Body.String(), `"resolution_changed":false`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	// Source project is checked after row lock and before any note write.
	m.ExpectBegin()
	m.ExpectQuery("SELECT .* FROM `crashes`.*FOR UPDATE").WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"id", "project", "resolved"}).AddRow(7, "necrovale", false))
	m.ExpectRollback()
	w = noteRequest(r, "POST", "report-notes/crash/7?project=cursemark", `{"request_key":"test-lock","note":"context"}`)
	if w.Code != 404 {
		t.Fatalf("scope %d %s", w.Code, w.Body)
	}
	// Legacy untagged scope is explicit rather than inferred from a game/default.
	m.ExpectQuery("SELECT .* FROM `feedbacks`.*LIMIT 1").WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"id", "project", "resolved"}))
	w = noteRequest(r, "GET", "report-notes/feedback/7?project=", "")
	if w.Code != 404 {
		t.Fatalf("legacy explicit scope %d %s", w.Code, w.Body)
	}
}

func noteRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/internal/mcp/"+path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 32))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestReportNoteInputAndRoutesFailClosed(t *testing.T) {
	r, _ := mcpMockDB(t)
	for _, path := range []string{"report-notes/crash/1", "report-notes/other/1?project=cursemark", "report-notes/crash/0?project=cursemark", "report-notes/crash/1?project=cursemark&limit=51", "report-notes/crash/1?project=cursemark&after_id=-1"} {
		if w := noteRequest(r, "GET", path, ""); w.Code != 400 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	for _, body := range []string{`{"note":"x","request_key":"request-1","actor":"Casey"}`, `{"note":"x","request_key":"request-1"} {}`, `{"note":"x","request_key":"request-1","state":"resolved"}`, `{"note":"","request_key":"request-1"}`} {
		if w := noteRequest(r, "POST", "report-notes/feedback/1?project=cursemark", body); w.Code != 400 {
			t.Fatalf("invalid body %d %s", w.Code, w.Body)
		}
	}
	for _, in := range []models.AppendReportNoteInput{{RequestKey: "request-1", Note: strings.Repeat("界", 2001)}, {RequestKey: "x", Note: "x"}, {RequestKey: "request-1", Note: "x", ReleaseState: "shipped"}, {RequestKey: "request-1", Note: "x", Evidence: &models.NoteEvidenceInput{Name: "../file", Content: "x", SHA256: "x"}}} {
		if models.ValidateReportNote(in) == nil {
			t.Fatal("invalid accepted")
		}
	}
}
func TestReportNotesMySQLAppendHistoryScopeAndRetry(t *testing.T) {
	db := syncTestDB(t)
	if err := models.MigrateReportNotes(db); err != nil {
		t.Fatal(err)
	}
	f := models.Feedback{Project: "cursemark", Message: "original evidence", Resolved: true}
	if err := db.Create(&f).Error; err != nil {
		t.Fatal(err)
	}
	c1 := models.Crash{Project: "cursemark", Message: "same group", GroupKey: "same", Resolved: false}
	c2 := models.Crash{Project: "cursemark", Message: "same group", GroupKey: "same", Resolved: true}
	db.Create(&c1)
	db.Create(&c2)
	raw := "# Full original findings\n\nα\n<untrusted>\n"
	in := models.AppendReportNoteInput{RequestKey: "first-note", Note: "Tested locally; unreleased. Owner closes manually.", State: "fixed", ReleaseState: "local", Evidence: &models.NoteEvidenceInput{Name: "findings.md", Content: raw, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))}}
	first, replay, err := models.AppendReportNote(db, "feedback", f.ID, "cursemark", "mcp", in)
	if err != nil || replay || first.Actor != "mcp" || first.CreatedAt.IsZero() || first.Evidence == nil || first.Evidence.Content != "" {
		t.Fatalf("append %+v %v %v", first, replay, err)
	}
	// Exact repeated attempts, including concurrent retry, must yield one durable record.
	var wg sync.WaitGroup
	fail := make(chan error, 16)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, r, e := models.AppendReportNote(db, "feedback", f.ID, "cursemark", "mcp", in)
			if e != nil || !r || n.ID != first.ID {
				fail <- fmt.Errorf("retry %d %v %v", n.ID, r, e)
			}
		}()
	}
	wg.Wait()
	close(fail)
	for e := range fail {
		t.Error(e)
	}
	var count int64
	db.Model(&models.ReportNote{}).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate retries %d", count)
	}
	changed := in
	changed.Note = "different"
	if _, _, e := models.AppendReportNote(db, "feedback", f.ID, "cursemark", "mcp", changed); e != models.ErrNoteConflict {
		t.Fatalf("conflict %v", e)
	}
	if _, _, e := models.AppendReportNote(db, "feedback", f.ID, "necrovale", "mcp", in); e == nil {
		t.Fatal("cross project append")
	}
	fail2 := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, e := models.AppendReportNote(db, "feedback", f.ID, "cursemark", "admin", models.AppendReportNoteInput{RequestKey: fmt.Sprintf("parallel-%d", i), Note: fmt.Sprintf("Context %d", i)})
			if e != nil {
				fail2 <- e
			}
		}(i)
	}
	wg.Wait()
	close(fail2)
	for e := range fail2 {
		t.Error(e)
	}
	// Pre-cutover notes are fixtures: current persistence rejects new per-report crash notes.
	for _, c := range []models.Crash{c1, c2} {
		if e := db.Create(&models.ReportNote{Kind: "crash", ReportID: c.ID, Project: "cursemark", Actor: "mcp", RequestKey: "crash-note", Note: "Underlying fault not reproduced; diagnostic only.", State: "blocked", ReleaseState: "local"}).Error; e != nil {
			t.Fatal(e)
		}
	}

	// Migration replay preserves complete contents and existing resolution/rows.
	for i := 0; i < 2; i++ {
		if e := models.MigrateReportNotes(db); e != nil {
			t.Fatal(e)
		}
	}
	var stored models.Feedback
	db.First(&stored, f.ID)
	if !stored.Resolved || stored.Message != f.Message {
		t.Fatal("notes altered source")
	}
	db.First(&c1, c1.ID)
	db.First(&c2, c2.ID)
	if c1.Resolved || !c2.Resolved {
		t.Fatal("crash state altered")
	}
	var e models.ReportNoteEvidence
	db.First(&e, first.Evidence.ID)
	if e.Content != raw || e.SHA256 != in.Evidence.SHA256 {
		t.Fatal("evidence changed")
	}
	t.Setenv("MCP_UPSTREAM_TOKEN", strings.Repeat("a", 32))
	r := gin.New()
	RegisterMCPRoutes(r)
	w := noteRequest(r, "GET", fmt.Sprintf("report-notes/feedback/%d?project=cursemark&limit=2", f.ID), "")
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	var data struct {
		Items []models.ReportNote `json:"items"`
		More  bool                `json:"has_more"`
		Next  uint                `json:"next_after_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &data)
	if len(data.Items) != 2 || !data.More || data.Next <= first.ID || strings.Contains(w.Body.String(), "<untrusted>") || strings.Contains(w.Body.String(), `"evidence"`) {
		t.Fatalf("page %s", w.Body)
	}
	w = noteRequest(r, "GET", fmt.Sprintf("report-notes/feedback/%d?project=necrovale", f.ID), "")
	if w.Code != 404 {
		t.Fatal("scope read", w.Code)
	}
	w = noteRequest(r, "GET", fmt.Sprintf("report-evidence/%d?project=cursemark", e.ID), "")
	if w.Code != 404 {
		t.Fatal("archive endpoint must not be published")
	}
	w = noteRequest(r, "GET", "report-notes?kind=feedback&project=cursemark", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"resolved":true`) {
		t.Fatalf("resolved index %s", w.Body)
	}
	w = noteRequest(r, "GET", fmt.Sprintf("report-notes/crash/%d?project=cursemark", c1.ID), "")
	json.Unmarshal(w.Body.Bytes(), &data)
	if w.Code != 200 || len(data.Items) != 1 || data.Items[0].ReportID != c1.ID {
		t.Fatal("group notes merged", w.Body)
	}
}
