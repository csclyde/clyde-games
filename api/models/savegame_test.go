package models

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestFeedbackSaveAssociationRequiresLockedMatchingOpenGameReport(t *testing.T) {
	for _, tc := range []struct {
		name, pid, project, source string
		resolved, valid            bool
	}{
		{"matching", "installation", "Cursemark", "", false, true},
		{"different installation", "other", "cursemark", "", false, false},
		{"installation case is exact", "Installation", "cursemark", "", false, false},
		{"different project", "installation", "necrovale", "", false, false},
		{"resolved", "installation", "cursemark", "", true, false},
		{"community source", "installation", "cursemark", "discord", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, m := cutoffMock(t)
			m.ExpectQuery("SELECT .* FROM `feedbacks`.*FOR UPDATE").WithArgs(7).
				WillReturnRows(sqlmock.NewRows([]string{"id", "p_id", "project", "source", "resolved"}).AddRow(7, tc.pid, tc.project, tc.source, tc.resolved))
			err := validateFeedbackSaveAssociation(db, Savegame{FeedbackID: 7, PID: "installation", Project: "cursemark"})
			if (err == nil) != tc.valid || (!tc.valid && !errors.Is(err, ErrFeedbackSaveAssociation)) {
				t.Fatal(err)
			}
		})
	}
	for _, pid := range []string{"", "  ", "unknown", "UNKNOWN"} {
		db, _ := cutoffMock(t)
		if e := validateFeedbackSaveAssociation(db, Savegame{FeedbackID: 7, PID: pid, Project: "cursemark"}); !errors.Is(e, ErrFeedbackSaveAssociation) {
			t.Fatal("anonymous association accepted", e)
		}
	}
	for _, missing := range []bool{true, false} {
		db, m := cutoffMock(t)
		query := m.ExpectQuery("SELECT .* FROM `feedbacks`.*FOR UPDATE").WithArgs(7)
		failure := errors.New("fixture database unavailable")
		if missing {
			query.WillReturnRows(sqlmock.NewRows([]string{"id"}))
		} else {
			query.WillReturnError(failure)
		}
		err := validateFeedbackSaveAssociation(db, Savegame{FeedbackID: 7, PID: "installation", Project: "cursemark"})
		if missing && !errors.Is(err, ErrFeedbackSaveAssociation) {
			t.Fatal(err)
		}
		if !missing && !errors.Is(err, failure) {
			t.Fatal("database failure hidden", err)
		}
	}
}
