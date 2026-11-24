package endpoints

import (
	"api.clyde.games/models"
	"testing"
)

func TestInvestigationRejectsInvalidScopesWithoutQuery(t *testing.T) {
	r, _ := mcpMockDB(t)
	for _, q := range []string{"kind=feedback&ids=1", "kind=other&project=cursemark&ids=1", "kind=crash&project=cursemark&ids=0", "kind=crash&project=cursemark&ids=1,1", "kind=feedback&project=cursemark&ids=1,-2", "kind=feedback&project=cursemark&ids="} {
		w := noteRequest(r, "GET", "report-investigations?"+q, "")
		if w.Code != 400 {
			t.Fatalf("%s: %d %s", q, w.Code, w.Body)
		}
	}
}
func TestInvestigationMySQLStatusIsDerivedAndNeverCloses(t *testing.T) {
	db := syncTestDB(t)
	if err := models.MigrateReportNotes(db); err != nil {
		t.Fatal(err)
	}
	f := models.Feedback{Project: "cursemark", Message: "source", Resolved: false}
	other := models.Feedback{Project: "necrovale", Message: "other"}
	db.Create(&f)
	db.Create(&other)
	appendNote := func(key, text, state, release string) {
		t.Helper()
		if _, _, e := models.AppendReportNote(db, "feedback", f.ID, "cursemark", "mcp", models.AppendReportNoteInput{RequestKey: key, Note: text, State: state, ReleaseState: release}); e != nil {
			t.Fatal(e)
		}
	}
	appendNote("legacy-fixed", "Tested local fix; needs owner review.", "fixed", "local")
	appendNote("context-only", "Later useful context.", "", "")
	status, e := models.ReadInvestigationStatuses(db, "feedback", "cursemark", []uint{f.ID}, nil)
	if e != nil || len(status) != 1 || status[0].State != "ready_for_review" || status[0].ReleaseState != "local" || status[0].NoteCount != 2 {
		t.Fatalf("derived %+v %v", status, e)
	}
	var original models.ReportNote
	db.Where("request_key = ?", "legacy-fixed").First(&original)
	if original.State != "fixed" {
		t.Fatal("rewrote history")
	}
	appendNote("confirm-new", "Player combination still needs confirmation.", "needs_confirmation", "")
	status, e = models.ReadInvestigationStatuses(db, "feedback", "cursemark", []uint{f.ID}, nil)
	if e != nil || status[0].State != "needs_confirmation" || status[0].ReleaseState != "local" || status[0].NoteCount != 3 {
		t.Fatal(status, e)
	}
	appendNote("ready-again", "Exact combination now tested locally; owner to review.", "ready_for_review", "")
	status, e = models.ReadInvestigationStatuses(db, "feedback", "cursemark", []uint{f.ID}, nil)
	if e != nil || status[0].State != "ready_for_review" {
		t.Fatal(status, e)
	}
	if _, e = models.ReadInvestigationStatuses(db, "feedback", "cursemark", []uint{other.ID}, nil); e == nil {
		t.Fatal("cross project")
	}
	status, e = models.ReadInvestigationStatuses(db, "feedback", "necrovale", []uint{other.ID}, nil)
	if e != nil || status[0].State != "unreviewed" || status[0].NoteCount != 0 {
		t.Fatal(status, e)
	}
	var check models.Feedback
	db.First(&check, f.ID)
	if check.Resolved || check.Message != "source" {
		t.Fatal("status changed source/resolution")
	}
	// Same-group crash identities remain individually statused.
	a := models.Crash{Project: "cursemark", GroupKey: "same", Resolved: false}
	b := a
	db.Create(&a)
	db.Create(&b)
	db.Create(&models.ReportNote{Kind: "crash", ReportID: a.ID, Project: "cursemark", Actor: "mcp", RequestKey: "crash-blocked", Note: "Cannot reproduce.", State: "blocked"})
	status, e = models.ReadInvestigationStatuses(db, "crash", "cursemark", []uint{a.ID, b.ID}, nil)
	if e != nil || status[0].State != "blocked" || status[1].State != "unreviewed" {
		t.Fatal("merged statuses", status, e)
	}
}
func TestInvestigationStateCompatibility(t *testing.T) {
	for in, want := range map[string]string{"": "unreviewed", "fixed": "ready_for_review", "ready_for_review": "ready_for_review", "blocked": "blocked", "needs_confirmation": "needs_confirmation", "investigating": "investigating"} {
		if got := models.CanonicalInvestigationState(in); got != want {
			t.Errorf("%s %s", in, got)
		}
	}
}
