package endpoints

import (
	"api.clyde.games/models"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCrashWorkAVLifecycleUsesRealIncidentsAndLegacyConflicts(t *testing.T) {
	db := workDB(t)
	now := time.Now().UTC()
	older := now.Add(-2 * time.Hour)
	later := now.Add(-time.Hour)
	middle := now.Add(-90 * time.Minute)
	a := workReport(t, db, "cursemark", "a", "Access violation A", true)
	db.Model(&a).Update("resolved_at", older)
	open := workReport(t, db, "cursemark", "a", "Access violation A", false)
	db.Model(&open).Update("ingested_at", middle)
	b := workReport(t, db, "cursemark", "b", "Access violation B", true)
	db.Model(&b).Update("resolved_at", later)
	// A returned before a DIFFERENT underlying incident was resolved. Comparing
	// against the family's maximum resolution would incorrectly hide this return.
	v, e := models.GetCrashWork(db, open.ID, "cursemark")
	if e != nil || !v.Returned || !v.NeedsReview {
		t.Fatalf("lost real AV return %+v %v", v, e)
	}
	claim, e := models.ClaimCrashWork(db, open.ID, "cursemark", "av-review", "")
	if e != nil || !claim.Acquired {
		t.Fatal(e)
	}
	workAppend(t, db, open, claim.Token, "av-outcome", "blocked")
	done, e := models.GetCrashWork(db, b.ID, "cursemark")
	if e != nil || done.Key != v.Key || done.NeedsReview || !done.Returned {
		t.Fatalf("historical return retriggered work %+v %v", done, e)
	}
	newer := workReport(t, db, "cursemark", "b", "Access violation B", false)
	again, e := models.GetCrashWork(db, newer.ID, "cursemark")
	if e != nil || again.Key != v.Key || !again.NeedsReview {
		t.Fatal("AV split/new evidence dropped", e)
	}
	log, e := models.ReadCrashInvestigationLog(db, open.ID, "cursemark", "", 0, 50)
	if e != nil || !log.ReportIndexComplete || len(log.ReportIDs) != 4 {
		t.Fatal("AV evidence membership missing", e)
	}
	// Pre-existing contradictory per-report states become ONE group needing
	// confirmation; the original states are preserved, never silently called fixed.
	x := workReport(t, db, "cursemark", "conflict", "fault", false)
	y := workReport(t, db, "cursemark", "conflict", "fault", false)
	for i, r := range []models.Crash{x, y} {
		state := "blocked"
		if i == 1 {
			state = "fixed"
		}
		if e := db.Create(&models.ReportNote{Kind: "crash", ReportID: r.ID, Project: "cursemark", Actor: "mcp", RequestKey: "legacy-state", State: state, Note: "Preserved original finding."}).Error; e != nil {
			t.Fatal(e)
		}
	}
	conflict, e := models.GetCrashWork(db, x.ID, "cursemark")
	if e != nil || !conflict.LegacyStateConflict || !conflict.NeedsReview || conflict.State != "needs_confirmation" {
		t.Fatalf("silently fixed contradictory history %+v %v", conflict, e)
	}
}
func TestCrashWorkConcurrentCompletionMigrationAndClosedHistory(t *testing.T) {
	db := workDB(t)
	a := workReport(t, db, "cursemark", "same", "fault", false)
	b := workReport(t, db, "cursemark", "same", "fault", false)
	claim, e := models.ClaimCrashWork(db, a.ID, "cursemark", "run-retry", "")
	if e != nil || !claim.Acquired {
		t.Fatal(e)
	}
	in := models.AppendCrashWorkInput{RequestKey: "same-outcome", Note: "Local tested fix; owner review pending, unreleased.", State: "ready_for_review", ReleaseState: "local", ClaimToken: claim.Token, Complete: true}
	var wg sync.WaitGroup
	ids := make(chan uint, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := a.ID
			if i%2 == 1 {
				id = b.ID
			}
			n, _, e := models.AppendCrashWork(db, id, "cursemark", "mcp", in)
			if e != nil {
				errs <- e
			} else {
				ids <- n.ID
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	close(ids)
	var first uint
	for id := range ids {
		if first == 0 {
			first = id
		}
		if id != first {
			t.Fatal("duplicate canonical outcomes")
		}
	}
	if first == 0 {
		t.Fatal("no saved outcome")
	}
	before, e := models.GetCrashWork(db, a.ID, "cursemark")
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e := models.MigrateCrashInvestigations(db); e != nil {
			t.Fatal(e)
		}
		if e := models.MigrateReportNotes(db); e != nil {
			t.Fatal(e)
		}
	}
	after, e := models.GetCrashWork(db, b.ID, "cursemark")
	if e != nil || after.Key != before.Key || after.State != before.State || after.CoveredThroughID != before.CoveredThroughID || after.NoteCount != 1 || after.NeedsReview {
		t.Fatalf("migration changed canonical record %+v %v", after, e)
	}
	if _, e := models.ResolveCrashID(db, uint64(a.ID)); e != nil {
		t.Fatal(e)
	}
	log, e := models.ReadCrashInvestigationLog(db, b.ID, "cursemark", "", 0, 50)
	if e != nil || len(log.Items) != 1 || log.Investigation.OpenReportCount != 0 {
		t.Fatal("closed history disappeared", e)
	}
	retry, e := models.ClaimCrashWork(db, b.ID, "cursemark", "run-retry", "")
	if e != nil || retry.Acquired {
		t.Fatal(fmt.Sprint("completed claim restarted: ", e))
	}
}
