package endpoints

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api.clyde.games/models"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestPortalFeedbackClosure(t *testing.T) {
	for _, tt := range []struct {
		name, source, disposition string
		closed, pending           bool
	}{
		{"discord resolve", "discord", "resolved", false, true},
		{"discord ignore", "discord", "ignored", false, false},
		{"steam resolve", "steam_reviews", "resolved", false, false},
		{"duplicate or conflicting closure", "discord", "ignored", true, false},
		{"legacy ticket closure", "discord", "", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router, mock := mcpMockDB(t)
			router.POST("/feedback/close", CloseFeedback)
			router.POST("/resolvefeedback", ResolveFeedback)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .*feedbacks.*FOR UPDATE").WithArgs("7").WillReturnRows(
				sqlmock.NewRows([]string{"id", "resolved", "source", "source_id", "closure_disposition"}).
					AddRow(7, tt.closed, tt.source, "12:34:56", map[bool]string{true: "resolved"}[tt.closed]),
			)
			want := tt.disposition
			path := "/feedback/close?id=7"
			if want == "" {
				want = "resolved"
				path = "/resolvefeedback?id=7"
			}
			if !tt.closed {
				if tt.pending {
					mock.ExpectExec("UPDATE .*feedbacks.*closure_disposition.*completion_reaction_due.*resolved").
						WithArgs(want, sqlmock.AnyArg(), true, sqlmock.AnyArg(), 7).WillReturnResult(sqlmock.NewResult(0, 1))
				} else {
					mock.ExpectExec("UPDATE .*feedbacks.*closure_disposition.*resolved").
						WithArgs(want, true, sqlmock.AnyArg(), 7).WillReturnResult(sqlmock.NewResult(0, 1))
				}
				mock.ExpectExec("UPDATE .*savegames.*feedback_id").WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				want = "resolved"
			}
			mock.ExpectCommit()
			req := httptest.NewRequest("POST", path, strings.NewReader(`{"disposition":"`+tt.disposition+`"}`))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != 200 || !strings.Contains(res.Body.String(), `"ClosureDisposition": "`+want+`"`) {
				t.Fatalf("closure: %d %s", res.Code, res.Body)
			}
		})
	}
}

func TestDiscordCompletionQueueRetriesAndAcknowledges(t *testing.T) {
	_, mock := mcpMockDB(t)
	now := time.Now().UTC()
	for _, transient := range []bool{true, false} {
		mock.ExpectQuery("SELECT .*feedbacks.*completion_reaction_due <= .*LIMIT 20").
			WithArgs(true, "resolved", "discord", now).
			WillReturnRows(sqlmock.NewRows([]string{"id", "source", "source_id"}).AddRow(7, "discord", "12:34:56"))
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE .*feedbacks.*SET .*completion_reaction_due.*id = .*completion_reaction_due <= ").
			WithArgs(sqlmock.AnyArg(), 7, true, "resolved", "discord", now).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		if !transient {
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE .*feedbacks.*completion_reaction_due.*completion_reaction_sent_at").
				WithArgs(nil, sqlmock.AnyArg(), sqlmock.AnyArg(), 7).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
		}
		attempts := 0
		err := deliverDiscordCompletions(models.AnalyticsDB, now, func(_ context.Context, target [3]string) error {
			attempts++
			if target != [3]string{"12", "34", "56"} {
				t.Fatalf("unexpected target %v", target)
			}
			if transient {
				return errors.New("bridge temporarily unavailable")
			}
			return nil
		})
		if err != nil || attempts != 1 {
			t.Fatalf("delivery result: attempts %d err %v", attempts, err)
		}
		now = now.Add(6 * time.Minute)
	}
}
