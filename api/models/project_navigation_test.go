package models

import (
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestNavigationActivityProjectScopeAndIncompleteSnapshot(t *testing.T) {
	db, m := cutoffMock(t)
	old := AnalyticsDB
	AnalyticsDB = db
	defer func() { AnalyticsDB = old }()
	m.ExpectQuery("SELECT .* FROM `projects` ORDER BY name ASC, id ASC").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("cursemark", "Cursemark").AddRow("necrovale", "Necrovale").AddRow("empty", "Empty"))
	m.ExpectQuery("SELECT project, COUNT.* FROM `feedbacks` WHERE resolved != true AND parent_feedback_id IS NULL AND `feedbacks`.`deleted_at` IS NULL GROUP BY `project`").WillReturnRows(sqlmock.NewRows([]string{"project", "count"}).AddRow("Cursemark", 2).AddRow("necrovale", 1))
	m.ExpectQuery("SELECT project, COUNT.* FROM `crashes` WHERE resolved != true AND `crashes`.`deleted_at` IS NULL GROUP BY `project`").WillReturnRows(sqlmock.NewRows([]string{"project", "count"}).AddRow("cursemark", 3))
	m.ExpectQuery("SELECT id, project FROM `feedbacks` WHERE resolved != true AND parent_feedback_id IS NULL AND `feedbacks`.`deleted_at` IS NULL ORDER BY id DESC LIMIT 5001").WillReturnRows(sqlmock.NewRows([]string{"id", "project"}).AddRow("18446744073709551615", "Cursemark").AddRow(21, "cursemark"))
	data, e := SelectProjectNavigation()
	if e != nil {
		t.Fatal(e)
	}
	if len(data) != 3 || data[0].OpenFeedback != 2 || data[0].OpenCrashes != 3 || !data[0].FeedbackIDsComplete || len(data[0].OpenFeedbackIDs) != 2 || data[0].OpenFeedbackIDs[0] != "18446744073709551615" {
		t.Fatal("activity identity/precision/scope lost")
	}
	if data[1].FeedbackIDsComplete || data[1].OpenFeedback != 1 || len(data[1].OpenFeedbackIDs) != 0 {
		t.Fatal("inconsistent count/ID snapshot falsely complete")
	}
	if !data[2].FeedbackIDsComplete || data[2].OpenFeedbackIDs == nil || len(data[2].OpenFeedbackIDs) != 0 {
		t.Fatal("empty source not represented completely")
	}
}

func TestNavigationActivityBoundIsHonest(t *testing.T) {
	db, m := cutoffMock(t)
	old := AnalyticsDB
	AnalyticsDB = db
	defer func() { AnalyticsDB = old }()
	m.ExpectQuery("SELECT .* FROM `projects`").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("cursemark", "Cursemark"))
	m.ExpectQuery("SELECT project, COUNT.* FROM `feedbacks`").WillReturnRows(sqlmock.NewRows([]string{"project", "count"}).AddRow("cursemark", 5001))
	m.ExpectQuery("SELECT project, COUNT.* FROM `crashes`").WillReturnRows(sqlmock.NewRows([]string{"project", "count"}))
	rows := sqlmock.NewRows([]string{"id", "project"})
	for i := 1; i <= 5001; i++ {
		rows.AddRow(i, "cursemark")
	}
	m.ExpectQuery("SELECT id, project FROM `feedbacks`.*LIMIT 5001").WillReturnRows(rows)
	data, e := SelectProjectNavigation()
	if e != nil {
		t.Fatal(e)
	}
	if data[0].FeedbackIDsComplete || len(data[0].OpenFeedbackIDs) != 5000 {
		t.Fatal("truncation hidden or unbounded")
	}
}
