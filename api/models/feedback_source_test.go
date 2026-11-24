package models

import (
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
	"time"
)

func sourceMockStart(t *testing.T) (sqlmock.Sqlmock, Feedback) {
	db, m := cutoffMock(t)
	old := AnalyticsDB
	AnalyticsDB = db
	t.Cleanup(func() { AnalyticsDB = old })
	m.ExpectBegin()
	m.ExpectExec("INSERT INTO `projects`").WillReturnResult(sqlmock.NewResult(0, 0))
	m.ExpectQuery("SELECT .* FROM `projects`.*FOR UPDATE").WithArgs("cursemark").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("cursemark"))
	return m, Feedback{Project: "Cursemark", Category: "Steam Discussion", Platform: "Steam", Message: "body\nhttps://steam.test/topic/"}
}
func TestSourceReplayPreservesResolvedAndDeleted(t *testing.T) {
	m, fb := sourceMockStart(t)
	m.ExpectQuery("SELECT .* FROM `feedbacks` WHERE source_key").WillReturnRows(sqlmock.NewRows([]string{"id", "resolved", "deleted_at"}).AddRow(42, true, time.Now()))
	m.ExpectCommit()
	added, err := AddSourceFeedback(fb, "steam_discussions", "topic", "https://steam.test/topic/")
	if err != nil || added {
		t.Fatalf("replay: added=%v error=%v", added, err)
	}
}
func TestLegacyLinkMatchesExactURLWithoutTouchingHistory(t *testing.T) {
	m, fb := sourceMockStart(t)
	m.ExpectQuery("SELECT .* FROM `feedbacks` WHERE source_key").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectQuery("SELECT .* FROM `feedbacks` WHERE project.*source_key IS NULL").WithArgs("cursemark", fb.Category, fb.Platform).WillReturnRows(sqlmock.NewRows([]string{"id", "message", "resolved"}).AddRow(10, "reply\nhttps://steam.test/topic/#c123", true).AddRow(11, fb.Message, true))
	m.ExpectExec("UPDATE `feedbacks` SET `ingested_at`=.*`source_key`=.*`source_url`=.* WHERE id = \\?").WithArgs(sqlmock.AnyArg(), "steam_discussions", "topic", sqlmock.AnyArg(), "https://steam.test/topic/", 11).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	added, err := AddSourceFeedback(fb, "steam_discussions", "topic", "https://steam.test/topic/")
	if err != nil || added {
		t.Fatalf("legacy: added=%v error=%v", added, err)
	}
}
func TestSourceInsertIsAtomicAndProjectScoped(t *testing.T) {
	m, fb := sourceMockStart(t)
	m.ExpectQuery("SELECT .* FROM `feedbacks` WHERE source_key").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectQuery("SELECT .* FROM `feedbacks` WHERE project.*source_key IS NULL").WithArgs("cursemark", fb.Category, fb.Platform).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectExec("INSERT INTO `feedbacks`").WillReturnResult(sqlmock.NewResult(12, 1))
	m.ExpectCommit()
	added, err := AddSourceFeedback(fb, "steam_discussions", "topic", "https://steam.test/topic/")
	if err != nil || !added {
		t.Fatalf("insert: added=%v error=%v", added, err)
	}
}
