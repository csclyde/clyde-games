package endpoints

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func TestSavegameFeedbackIDParsingAndUploadBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/savegame", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSavegameRequestBytes)
		save, err := readSavegame(c)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, savegameResponse(save))
	})
	for _, tc := range []struct {
		query  string
		status int
		id     uint
	}{
		{"", 201, 0}, {"feedback_id=42", 201, 42}, {"feedbackId=42", 201, 42},
		{"feedback_id=42&feedbackId=42", 201, 42},
		{"feedback_id=42&feedbackId=43", 400, 0}, {"feedback_id=0", 400, 0},
		{"feedback_id=-1", 400, 0}, {"feedback_id=garbage", 400, 0},
		{"feedback_id=18446744073709551616", 400, 0}, {"feedbackId=4x", 400, 0},
	} {
		t.Run(tc.query, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/savegame?"+tc.query, strings.NewReader("fixture"))
			req.Header.Set("Content-Type", "application/octet-stream")
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status %d: %s", w.Code, w.Body)
			}
			if tc.status == 201 {
				var save SavegameResponse
				if e := json.Unmarshal(w.Body.Bytes(), &save); e != nil || save.FeedbackID != tc.id {
					t.Fatal(save, e)
				}
			}
		})
	}
	for _, n := range []int{0, maxSavegameBytes, maxSavegameBytes + 1, maxSavegameRequestBytes + 1} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/savegame", bytes.NewReader(bytes.Repeat([]byte{7}, n))))
		want := 400
		if n == maxSavegameBytes {
			want = 201
		}
		if w.Code != want {
			t.Fatalf("size %d: status %d", n, w.Code)
		}
	}
	for _, n := range []int{maxSavegameBytes, maxSavegameBytes + 1} {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, e := writer.CreateFormFile("savegame", "fixture.sav")
		if e != nil {
			t.Fatal(e)
		}
		if _, e = io.Copy(part, bytes.NewReader(bytes.Repeat([]byte{8}, n))); e != nil {
			t.Fatal(e)
		}
		if e = writer.WriteField("feedbackId", "42"); e != nil {
			t.Fatal(e)
		}
		writer.Close()
		req := httptest.NewRequest("POST", "/savegame", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		want := 201
		if n > maxSavegameBytes {
			want = 400
		}
		if w.Code != want {
			t.Fatalf("multipart size %d status %d", n, w.Code)
		}
	}
}

func TestFeedbackSaveMySQLBlockedDiagnosticAssociationAndRetention(t *testing.T) {
	db := syncTestDB(t)
	if e := db.AutoMigrate(&models.Savegame{}, &models.BlockedUser{}, &models.Event{}, &models.EventItem{}); e != nil {
		t.Fatal(e)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/feedback", AddFeedback)
	r.POST("/savegame", AddSavegame)
	r.POST("/event", AddEvent)
	const pid = "feedback-fixture-installation"
	post := func(url, kind string, data []byte) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", url, bytes.NewReader(data))
		req.Header.Set("Content-Type", kind)
		r.ServeHTTP(w, req)
		return w
	}
	feedback := func(player, project string) models.Feedback {
		t.Helper()
		raw, _ := json.Marshal(models.Feedback{PID: player, Project: project, Message: "Synthetic local diagnostic", Rating: 3})
		w := post("/feedback", "application/json", raw)
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body)
		}
		var f models.Feedback
		if e := json.Unmarshal(w.Body.Bytes(), &f); e != nil {
			t.Fatal(e)
		}
		return f
	}
	attachment := func(f models.Feedback, body []byte) *httptest.ResponseRecorder {
		return post(fmt.Sprintf("/savegame?project=%s&pid=%s&feedback_id=%d&reason=feedback&build=2026-11-01", f.Project, f.PID, f.ID), "application/octet-stream", body)
	}
	ordinary := post("/savegame?pid="+pid+"&project=cursemark", "application/octet-stream", []byte("ordinary fixture"))
	if ordinary.Code != 201 {
		t.Fatal(ordinary.Code, ordinary.Body)
	}
	// Rows predating the feedback-ID column can be NULL rather than zero.
	legacy := models.Savegame{PID: pid, Project: "cursemark", Data: []byte("legacy ordinary fixture")}
	if e := db.Create(&legacy).Error; e != nil {
		t.Fatal(e)
	}
	if e := db.Model(&legacy).Update("feedback_id", nil).Error; e != nil {
		t.Fatal(e)
	}
	first := feedback(pid, "Cursemark")
	if w := attachment(first, []byte("normal diagnostic fixture")); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	if _, e := models.AddEvent(models.Event{PID: pid, Project: "cursemark", Items: []models.EventItem{{Item: "fixture"}}}); e != nil {
		t.Fatal(e)
	}
	// This is the actual server's altered-resource detection and block path.
	bad, _ := json.Marshal(models.Event{PID: pid, Project: "cursemark", Resources: models.EventResources{Essence: 200001}})
	if w := post("/event", "application/json", bad); w.Code != 403 {
		t.Fatal(w.Code, w.Body)
	}
	for _, model := range []interface{}{&models.Event{}, &models.EventItem{}} {
		var count int64
		if e := db.Unscoped().Model(model).Count(&count).Error; e != nil || count != 0 {
			t.Fatal("metrics survived block", count, e)
		}
	}
	var ordinaryCount, attachedCount int64
	db.Unscoped().Model(&models.Savegame{}).Where("feedback_id IS NULL OR feedback_id = 0").Count(&ordinaryCount)
	db.Unscoped().Model(&models.Savegame{}).Where("feedback_id <> 0").Count(&attachedCount)
	if ordinaryCount != 0 || attachedCount != 1 {
		t.Fatal("block cleanup removed feedback evidence or retained ordinary saves", ordinaryCount, attachedCount)
	}
	// Feedback itself and a same-installation/project attachment still work.
	second := feedback(pid, "cursemark")
	body := []byte(`{"resources":{"essence":200001},"fixture":"altered but valid diagnostic"}`)
	w := attachment(second, body)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	var response SavegameResponse
	if e := json.Unmarshal(w.Body.Bytes(), &response); e != nil {
		t.Fatal(e)
	}
	stored, e := models.SelectSavegame(fmt.Sprint(response.ID))
	if e != nil {
		t.Fatal(e)
	}
	digest := sha256.Sum256(body)
	if stored.FeedbackID != second.ID || !bytes.Equal(stored.Data, body) || stored.Hash != hex.EncodeToString(digest[:]) || stored.Size != uint32(len(body)) {
		t.Fatal("associated evidence changed")
	}
	open, e := models.SelectAllFeedback()
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, f := range open {
		if f.ID == second.ID {
			found = f.SavegameID == stored.ID && !f.Resolved
		}
	}
	if !found {
		t.Fatal("portal feedback attachment readback missing")
	}
	// A reason label is not an exception, nor are metrics made acceptable.
	if w := post("/savegame?pid="+pid+"&project=cursemark&reason=feedback", "application/octet-stream", body); w.Code != 403 {
		t.Fatal(w.Code, w.Body)
	}
	if w := post("/event", "application/json", []byte(`{"PID":"`+pid+`","Project":"cursemark","Resources":{"Essence":2}}`)); w.Code != 403 {
		t.Fatal(w.Code, w.Body)
	}
	other := feedback("other-installation", "cursemark")
	wrongProject := feedback(pid, "necrovale")
	closed := feedback(pid, "cursemark")
	db.Model(&closed).Update("resolved", true)
	deleted := feedback(pid, "cursemark")
	db.Delete(&deleted)
	community := feedback(pid, "cursemark")
	db.Model(&community).Update("source", "discord")
	for _, query := range []string{
		fmt.Sprintf("pid=%s&project=cursemark&feedback_id=%d", pid, other.ID),
		fmt.Sprintf("pid=%s&project=cursemark&feedback_id=%d", pid, wrongProject.ID),
		fmt.Sprintf("pid=%s&project=cursemark&feedback_id=%d", pid, closed.ID),
		fmt.Sprintf("pid=%s&project=cursemark&feedback_id=%d", pid, deleted.ID),
		fmt.Sprintf("pid=%s&project=cursemark&feedback_id=%d", pid, community.ID),
		fmt.Sprintf("pid=%s&project=cursemark&feedback_id=999999", pid),
		fmt.Sprintf("pid=unknown&project=cursemark&feedback_id=%d", second.ID),
		fmt.Sprintf("project=cursemark&feedback_id=%d", second.ID),
		fmt.Sprintf("pid=%s&feedback_id=%d", pid, second.ID),
	} {
		if w := post("/savegame?"+query, "application/octet-stream", body); w.Code != 400 {
			t.Fatalf("association %s status %d", query, w.Code)
		}
	}
	// The exception retains the project-specific telemetry build floor.
	if _, e := models.SaveOldestEventBuild("2026-11-01", "cursemark"); e != nil {
		t.Fatal(e)
	}
	if w := post(fmt.Sprintf("/savegame?pid=%s&project=cursemark&feedback_id=%d&build=2026-10-01", pid, second.ID), "application/octet-stream", body); w.Code != 400 {
		t.Fatal("build floor bypassed", w.Code, w.Body)
	}
	var all int64
	db.Unscoped().Model(&models.Savegame{}).Count(&all)
	if all != 2 {
		t.Fatal("rejections created rows", all)
	}
	// Repeated/manual blocking must also retain all diagnostic attachments.
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- models.BlockUser(pid) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	db.Unscoped().Model(&models.Savegame{}).Count(&all)
	if all != 2 {
		t.Fatal("repeated blocking deleted evidence", all)
	}
	// A repeated block racing an attachment cannot delete feedback evidence.
	racing := feedback(pid, "cursemark")
	errs = make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); errs <- models.BlockUser(pid) }()
	go func() {
		defer wg.Done()
		_, e := models.AddSavegame(models.Savegame{PID: pid, Project: "cursemark", FeedbackID: racing.ID, Build: "2026-11-01", Data: body})
		errs <- e
	}()
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var raceCount int64
	db.Model(&models.Savegame{}).Where("feedback_id = ?", racing.ID).Count(&raceCount)
	if raceCount != 1 {
		t.Fatal("concurrent block lost new attachment", raceCount)
	}
	var check models.Feedback
	if e := db.First(&check, second.ID).Error; e != nil || check.Resolved {
		t.Fatal("feedback was resolved or lost", e)
	}
}
