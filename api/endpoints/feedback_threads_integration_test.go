package endpoints

import (
	"api.clyde.games/models"
	"fmt"
	"testing"
	"time"
)

func TestSteamThreadLifecycleMySQL(t *testing.T) {
	db := syncTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	base := "https://steamcommunity.com/app/3219180/discussions/0/1234/"
	makeItem := func(link string, at time.Time) models.Feedback {
		f := models.Feedback{Project: "cursemark", Message: "Steam discussion: Example\n\nBody\n" + link, Category: "Steam Discussion", Platform: "Steam"}
		f.CreatedAt = at
		return f
	}
	root := makeItem(base, now.Add(-time.Hour))
	root.Resolved = true
	reply := makeItem(base+"#c123", now.Add(-time.Minute))
	// Historical rows retain IDs and resolution when linked.
	if e := db.Create(&root).Error; e != nil {
		t.Fatal(e)
	}
	if e := db.Create(&reply).Error; e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e := models.BackfillSteamFeedbackThreads(); e != nil {
			t.Fatal(e)
		}
	}
	var stored models.Feedback
	if e := db.First(&stored, reply.ID).Error; e != nil || stored.ParentFeedbackID == nil || *stored.ParentFeedbackID != root.ID {
		t.Fatalf("legacy reply not linked: %v %+v", e, stored.ParentFeedbackID)
	}
	dup := makeItem(base+"#c123", reply.CreatedAt)
	dup.Markdown = "> Quoted reply"
	root.ID = 0
	var r models.Feedback
	assertResolved := func(want bool) {
		t.Helper()
		if e := db.Where("source_id = ?", base).First(&r).Error; e != nil || r.Resolved != want {
			t.Fatalf("resolved=%v want=%v err=%v", r.Resolved, want, e)
		}
	}
	assertResolved(true)
	added, skip, e := models.ImportSteamFeedbackThread(root, []models.Feedback{dup}, false)
	if e != nil || added != 0 || skip != 1 {
		t.Fatalf("duplicate: %d %d %v", added, skip, e)
	}
	assertResolved(true)
	if e := db.First(&stored, reply.ID).Error; e != nil || stored.Markdown != dup.Markdown || stored.Message != reply.Message || !stored.UpdatedAt.Equal(reply.UpdatedAt) {
		t.Fatalf("format recovery changed original evidence or timestamps: %v", e)
	}
	later := makeItem(base+"#c124", now)
	earlier := makeItem(base+"#c125", now.Add(-30*time.Second))
	added, _, e = models.ImportSteamFeedbackThread(root, []models.Feedback{later, earlier}, false)
	if e != nil || added != 2 {
		t.Fatalf("imports: %d %v", added, e)
	}
	assertResolved(false)
	if e := db.AutoMigrate(&models.Savegame{}); e != nil {
		t.Fatal(e)
	}
	list, e := models.SelectAllFeedback()
	if e != nil || len(list) != 1 || len(list[0].Replies) != 3 || list[0].Replies[1].SourceID != base+"#c125" || list[0].Replies[2].SourceID != base+"#c124" {
		t.Fatalf("chronological list: %+v (%v)", list, e)
	}
	if _, e := models.ResolveFeedback(fmt.Sprint(r.ID)); e != nil {
		t.Fatal(e)
	}
	added, _, e = models.ImportSteamFeedbackThread(root, []models.Feedback{earlier, later}, true)
	if e != nil || added != 0 {
		t.Fatalf("resync: %d %v", added, e)
	}
	assertResolved(true)
	other := makeItem("https://steamcommunity.com/app/3219180/discussions/0/5678/", now.Add(-time.Hour))
	otherReply := makeItem("https://steamcommunity.com/app/3219180/discussions/0/5678/#c222", now)
	added, _, e = models.ImportSteamFeedbackThread(other, []models.Feedback{otherReply}, false)
	if e != nil || added != 1 {
		t.Fatalf("older original post retained for new reply: %d %v", added, e)
	}
}
