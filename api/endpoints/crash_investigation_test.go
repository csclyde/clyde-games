package endpoints

import (
	"api.clyde.games/models"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestCrashInvestigationInputAndReadOnlyRoute(t *testing.T) {
	t.Setenv("PORTAL_AUTH_MODE", "legacy")
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("x", 32))
	r := gin.New()
	RegisterReportNoteRoutes(r)
	for _, path := range []string{"/crash-investigation/0?project=cursemark", "/crash-investigation/1", "/crash-investigation/1?project=Cursemark", "/crash-investigation/1?project=cursemark&after_id=-1", "/crash-investigation/1?project=cursemark&limit=51", "/crash-investigation/1?project=cursemark&family=arbitrary", "/crash-investigation/1?project=cursemark&family=&family=access_violations"} {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("X-Admin-API-Token", strings.Repeat("x", 32))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	for _, method := range []string{"POST", "PUT", "DELETE"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/crash-investigation/1?project=cursemark", nil))
		if w.Code != 404 {
			t.Fatalf("write route %s: %d", method, w.Code)
		}
	}
}
func TestCrashInvestigationMySQLScopeHistoryPaginationAndNoWrites(t *testing.T) {
	db := syncTestDB(t)
	if e := models.MigrateReportNotes(db); e != nil {
		t.Fatal(e)
	}
	rows := []models.Crash{
		{Project: "cursemark", GroupKey: "same", Message: "Null access", Count: 1},
		{Project: "Cursemark", GroupKey: "same", Message: "Null access", Resolved: true, Count: 7},
		{Project: "necrovale", GroupKey: "same", Message: "private other game", Count: 1},
		{Project: "cursemark", GroupKey: "different", Message: "other incident", Count: 1},
		{Project: "cursemark", Message: "legacy one", Count: 8},
		{Project: "cursemark", Message: "legacy two", Count: 9},
		{Project: "cursemark", GroupKey: "av-a", Message: "Access violation at a", Count: 1},
		{Project: "cursemark", GroupKey: "av-b", Message: "Access VIOLATION at b", Resolved: true, Count: 1},
		{Project: "necrovale", GroupKey: "av-a", Message: "Access violation elsewhere", Count: 1},
		{Project: "cursemark", GroupKey: "same", Message: "soft deleted", Count: 1},
		{Project: "cursemark", GroupKey: "SAME", Message: "case distinct identity", Count: 1},
	}
	for i := range rows {
		if e := db.Create(&rows[i]).Error; e != nil {
			t.Fatal(e)
		}
		n := models.ReportNote{Kind: "crash", ReportID: rows[i].ID, Project: models.NormalizeProjectID(rows[i].Project), Actor: "mcp", Note: rows[i].Message, RequestKey: "fixture-note"}
		if e := db.Create(&n).Error; e != nil {
			t.Fatal(e)
		}
	}
	if e := db.Delete(&rows[9]).Error; e != nil {
		t.Fatal(e)
	}
	// Even an inconsistent note row cannot leak another project's text.
	db.Create(&models.ReportNote{Kind: "crash", ReportID: rows[0].ID, Project: "necrovale", Note: "wrong note scope", RequestKey: "foreign-note"})
	db.Create(&models.ReportNote{Kind: "feedback", ReportID: rows[0].ID, Project: "cursemark", Note: "feedback is separate", RequestKey: "other-kind"})
	snapshot := func() string {
		var c []models.Crash
		var n []models.ReportNote
		db.Unscoped().Order("id").Find(&c)
		db.Order("id").Find(&n)
		b, _ := json.Marshal([]any{c, n})
		return string(b)
	}
	emptyAnchor := models.Crash{Project: "cursemark", GroupKey: "empty", Message: "No saved analysis", Count: 1}
	if e := db.Create(&emptyAnchor).Error; e != nil {
		t.Fatal(e)
	}
	before := snapshot()
	emptyLog, e := models.ReadCrashInvestigationLog(db, emptyAnchor.ID, "cursemark", "", 0, 20)
	if e != nil || emptyLog.NoteCount != 0 || emptyLog.Items == nil || len(emptyLog.Items) != 0 || emptyLog.ReportCount != 1 {
		t.Fatalf("empty log %+v %v", emptyLog, e)
	}
	log, e := models.ReadCrashInvestigationLog(db, rows[0].ID, "cursemark", "", 0, 1)
	if e != nil || log.ReportCount != 2 || log.OpenReportCount != 1 || log.NoteCount != 2 || len(log.Items) != 1 || !log.HasMore || log.Items[0].ReportID != rows[0].ID {
		t.Fatalf("first %+v %v", log, e)
	}
	next, e := models.ReadCrashInvestigationLog(db, rows[0].ID, "cursemark", "", log.NextAfterID, 1)
	if e != nil || next.HasMore || len(next.Items) != 1 || !next.Items[0].ReportResolved || next.Items[0].ReportID != rows[1].ID || next.FrontierID != log.FrontierID {
		t.Fatalf("history %+v %v", next, e)
	}
	if !next.IncludesResolvedReports {
		t.Fatal("history coverage missing")
	}
	empty, e := models.ReadCrashInvestigationLog(db, rows[0].ID, "cursemark", "", next.NextAfterID, 50)
	if e != nil || len(empty.Items) != 0 || empty.HasMore {
		t.Fatalf("tail %+v %v", empty, e)
	}
	for _, i := range []int{4, 5} {
		x, e := models.ReadCrashInvestigationLog(db, rows[i].ID, "cursemark", "", 0, 20)
		if e != nil || x.ReportCount != 1 || len(x.Items) != 1 || x.Items[0].ReportID != rows[i].ID {
			t.Fatalf("legacy identities merged %+v %v", x, e)
		}
	}
	av, e := models.ReadCrashInvestigationLog(db, rows[6].ID, "cursemark", "access_violations", 0, 50)
	if e != nil || av.GroupKey != "" || av.ReportCount != 2 || av.NoteCount != 2 || len(av.Items) != 2 || av.Items[0].ReportGroupKey == av.Items[1].ReportGroupKey {
		t.Fatalf("AV %+v %v", av, e)
	}
	if _, e = models.ReadCrashInvestigationLog(db, rows[0].ID, "cursemark", "access_violations", 0, 20); !errors.Is(e, models.ErrNoteInput) {
		t.Fatalf("non-AV family %v", e)
	}
	if _, e = models.ReadCrashInvestigationLog(db, rows[0].ID, "necrovale", "", 0, 20); !errors.Is(e, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign anchor %v", e)
	}
	if _, e = models.ReadCrashInvestigationLog(db, rows[9].ID, "cursemark", "", 0, 20); !errors.Is(e, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted anchor %v", e)
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("read projection changed source reports or notes")
	}
	// Exercise the authenticated portal handler against the same fixture database.
	t.Setenv("PORTAL_AUTH_MODE", "legacy")
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("x", 32))
	r := gin.New()
	RegisterReportNoteRoutes(r)
	req := httptest.NewRequest("GET", "/crash-investigation/"+strconv.FormatUint(uint64(rows[0].ID), 10)+"?project=cursemark&limit=1", nil)
	req.Header.Set("X-Admin-API-Token", strings.Repeat("x", 32))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"has_more":true`) {
		t.Fatalf("handler %d %s", w.Code, w.Body)
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("handler changed source records")
	}
}
