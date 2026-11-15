package endpoints

import (
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
)

func TestMCPFeedbackSave(t *testing.T) {
	r, mock := mcpMockDB(t)
	mock.ExpectQuery("SELECT .* FROM `savegames` WHERE feedback_id = \\?.*").WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "filename", "content_type", "size", "hash"}).AddRow(12, "../../save.json", "application/json", 2, "hash"))
	mock.ExpectQuery("SELECT OCTET_LENGTH\\(data\\) FROM `savegames` WHERE id = \\?.*").WithArgs(uint(12)).WillReturnRows(sqlmock.NewRows([]string{"size"}).AddRow(2))
	mock.ExpectQuery("SELECT `data` FROM `savegames` WHERE .*id = \\? AND OCTET_LENGTH\\(data\\) <= \\?.*").WithArgs(uint(12), maxSavegameBytes).WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte("{}")))
	w := mcpRequest(r, "GET", "feedback/7/save")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"data":"e30="`) || !strings.Contains(w.Body.String(), `"filename":"save.json"`) {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
}
func TestMCPFeedbackSaveOversize(t *testing.T) {
	r, mock := mcpMockDB(t)
	mock.ExpectQuery("SELECT .* FROM `savegames` WHERE feedback_id = \\?.*").WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(12))
	mock.ExpectQuery("SELECT OCTET_LENGTH\\(data\\) FROM `savegames` WHERE id = \\?.*").WithArgs(uint(12)).WillReturnRows(sqlmock.NewRows([]string{"size"}).AddRow(maxSavegameBytes + 1))
	w := mcpRequest(r, "GET", "feedback/7/save")
	if w.Code != 413 {
		t.Fatalf("oversize %d %s", w.Code, w.Body)
	}
}
