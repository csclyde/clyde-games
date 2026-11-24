package endpoints

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestUserHistoryMySQLNewestFirstTiesAndScope(t *testing.T) {
	db := syncTestDB(t)
	if e := db.AutoMigrate(&models.Savegame{}, &models.BlockedUser{}, &models.Event{}, &models.EventItem{}); e != nil {
		t.Fatal(e)
	}
	stamp := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	for id := uint(1); id <= 5; id++ {
		created := stamp
		if id == 1 {
			created = stamp.Add(-time.Hour)
		}
		if id >= 4 {
			created = stamp.Add(time.Hour)
		}
		pid := "history-player"
		if id == 4 {
			pid = "other-player"
		}
		model := gorm.Model{ID: id, CreatedAt: created, UpdatedAt: created}
		if id == 5 {
			model.DeletedAt = gorm.DeletedAt{Time: stamp, Valid: true}
		}
		for _, row := range []interface{}{
			&models.Feedback{Model: model, PID: pid, Project: "cursemark", Message: "fixture feedback", Resolved: id == 3},
			&models.Crash{Model: model, PID: pid, Project: "cursemark", Message: "fixture crash", Resolved: id == 3},
			&models.Savegame{Model: model, PID: pid, Project: "cursemark", Data: []byte("private save bytes"), Filename: "fixture.sav", FeedbackID: 3},
			&models.Event{Model: model, PID: pid, Project: "cursemark", Type: "fixture", Items: []models.EventItem{{Item: "fixture item"}}},
		} {
			if e := db.Create(row).Error; e != nil {
				t.Fatal(e)
			}
		}
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/user/:id", GetUserHistory)
	for _, pid := range []string{"history-player", "other-player"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/user/"+pid, nil))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
		var h models.UserHistory
		if e := json.Unmarshal(w.Body.Bytes(), &h); e != nil {
			t.Fatal(e)
		}
		want := []uint{3, 2, 1}
		if pid == "other-player" {
			want = []uint{4}
		}
		check := func(ids []uint) {
			t.Helper()
			if len(ids) != len(want) {
				t.Fatal(ids, want)
			}
			for i, id := range ids {
				if id != want[i] {
					t.Fatal(ids, want)
				}
			}
		}
		ids := []uint{}
		for _, x := range h.Feedback {
			ids = append(ids, x.ID)
		}
		check(ids)
		ids = nil
		for _, x := range h.Crashes {
			ids = append(ids, x.ID)
		}
		check(ids)
		ids = nil
		for _, x := range h.Savegames {
			if len(x.Data) != 0 {
				t.Fatal("history leaked save bytes")
			}
			ids = append(ids, x.ID)
		}
		check(ids)
		ids = nil
		for _, x := range h.Events {
			if len(x.Items) != 1 {
				t.Fatal("event detail preload lost")
			}
			ids = append(ids, x.ID)
		}
		check(ids)
		if pid == "history-player" && (!h.Feedback[0].Resolved || !h.Crashes[0].Resolved) {
			t.Fatal("resolved history hidden")
		}
	}
}
