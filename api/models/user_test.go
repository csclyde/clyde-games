package models

import (
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestUserHistoryQueriesNewestFirstWithStableTies(t *testing.T) {
	db, m := cutoffMock(t)
	old := AnalyticsDB
	AnalyticsDB = db
	defer func() { AnalyticsDB = old }()
	m.ExpectQuery("SELECT count.* FROM `blocked_users` WHERE p_id = ").WithArgs("history-player").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	for _, table := range []string{"feedbacks", "crashes", "savegames", "events"} {
		m.ExpectQuery("SELECT .* FROM `" + table + "` WHERE .* ORDER BY created_at DESC, id DESC").WithArgs("history-player").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	}
	h, err := SelectUserHistory("history-player")
	if err != nil || h == nil || h.Blocked || h.Feedback == nil || h.Crashes == nil || h.Savegames == nil || h.Events == nil {
		t.Fatal(h, err)
	}
}
