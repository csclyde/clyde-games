package endpoints

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api.clyde.games/models"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func mcpMockDB(t *testing.T) (*gin.Engine, sqlmock.Sqlmock) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	old := models.AnalyticsDB
	models.AnalyticsDB = db
	t.Cleanup(func() {
		models.AnalyticsDB = old
		conn.Close()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("MCP_UPSTREAM_TOKEN", strings.Repeat("a", 32))
	r := gin.New()
	RegisterMCPRoutes(r)
	return r, mock
}
func mcpRequest(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/internal/mcp/"+path, nil)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 32))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestMCPDatabaseListsAndDetail(t *testing.T) {
	r, mock := mcpMockDB(t)
	// Assert the MySQL dialect quotes the reserved commit column. Lists must
	// exclude soft-deleted/resolved rows and be oldest-first with a hard limit.
	ids := sqlmock.NewRows([]string{"id"})
	for i := 1; i <= 201; i++ {
		ids.AddRow(i)
	}
	mock.ExpectQuery("SELECT .*id.* FROM `feedbacks` WHERE deleted_at IS NULL AND resolved = \\? ORDER BY created_at ASC, id ASC LIMIT 100001").WithArgs(false).WillReturnRows(ids)
	query := "SELECT .*`commit`.* FROM `feedbacks` WHERE deleted_at IS NULL AND id IN .* ORDER BY created_at ASC, id ASC"
	rows := sqlmock.NewRows([]string{"id", "project", "message", "commit", "created_at"})
	for i := 1; i <= 200; i++ {
		rows.AddRow(i, "game", strings.Repeat("x", 600), "abcd", time.Now())
	}
	mock.ExpectQuery(query).WillReturnRows(rows)
	w := mcpRequest(r, "GET", "feedback")
	if w.Code != 200 {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	var result struct {
		ActiveIDs     []uint `json:"active_ids"`
		ActiveCount   int    `json:"active_count"`
		IndexComplete bool   `json:"index_complete"`
		Items         []mcpReport
		Truncated     bool
		Limit         int
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.ActiveIDs) != 201 || result.ActiveIDs[200] != 201 || result.ActiveCount != 201 || !result.IndexComplete || len(result.Items) != 200 || !result.Truncated || result.Limit != 200 || !result.Items[0].TextTruncated || len(result.Items[0].Message) != 512 {
		t.Fatalf("wrong bounded list: %+v", result)
	}
	mock.ExpectQuery("SELECT .*`commit`.* FROM `crashes` WHERE deleted_at IS NULL AND id = \\? LIMIT 1").WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"id", "message", "stack", "count", "resolved"}).AddRow(9, "crash", "trace", 3, false))
	w = mcpRequest(r, "GET", "crashes/9")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"stack":"trace"`) {
		t.Fatalf("detail: %d %s", w.Code, w.Body)
	}
	mock.ExpectQuery("SELECT .* FROM `feedbacks` WHERE deleted_at IS NULL AND id = \\? LIMIT 1").WithArgs(999).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	w = mcpRequest(r, "GET", "feedback/999")
	if w.Code != 404 {
		t.Fatalf("missing detail: %d %s", w.Code, w.Body)
	}
}
func TestMCPDatabaseEmptyList(t *testing.T) {
	r, mock := mcpMockDB(t)
	mock.ExpectQuery("SELECT .* FROM `crashes`").WithArgs(false).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	w := mcpRequest(r, "GET", "crashes")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) || !strings.Contains(w.Body.String(), `"truncated":false`) {
		t.Fatalf("empty list: %d %s", w.Code, w.Body)
	}
}
func expectFeedbackLookup(mock sqlmock.Sqlmock, resolved bool) {
	mock.ExpectQuery("SELECT .* FROM `feedbacks` WHERE `feedbacks`.`id` = \\? AND `feedbacks`.`deleted_at` IS NULL ORDER BY `feedbacks`.`id` LIMIT 1 FOR UPDATE").WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"id", "resolved"}).AddRow(7, resolved))
}
func TestMCPResolveFeedbackTransactionAndIdempotence(t *testing.T) {
	r, mock := mcpMockDB(t)
	for _, alreadyResolved := range []bool{false, true} {
		mock.ExpectBegin()
		expectFeedbackLookup(mock, alreadyResolved)
		mock.ExpectExec("UPDATE `feedbacks` SET `resolved`=\\?,`updated_at`=\\? WHERE `feedbacks`.`deleted_at` IS NULL AND `id` = \\?").WithArgs(true, sqlmock.AnyArg(), 7).WillReturnResult(sqlmock.NewResult(0, 1))
		// Soft-delete only: no DELETE FROM, and already-deleted saves are excluded.
		mock.ExpectExec("UPDATE `savegames` SET `deleted_at`=\\? WHERE feedback_id = \\? AND `savegames`.`deleted_at` IS NULL").WithArgs(sqlmock.AnyArg(), 7).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		w := mcpRequest(r, "POST", "feedback/7/resolve")
		if w.Code != 200 {
			t.Fatalf("resolve: %d %s", w.Code, w.Body)
		}
	}
}
func TestMCPResolveRollbackOnSaveFailure(t *testing.T) {
	r, mock := mcpMockDB(t)
	mock.ExpectBegin()
	expectFeedbackLookup(mock, false)
	mock.ExpectExec("UPDATE `feedbacks`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `savegames`").WillReturnError(errors.New("private db error"))
	mock.ExpectRollback()
	w := mcpRequest(r, "POST", "feedback/7/resolve")
	if w.Code != 500 || strings.Contains(w.Body.String(), "private db error") {
		t.Fatalf("rollback: %d %s", w.Code, w.Body)
	}
}
func TestMCPResolveMissingRollsBack(t *testing.T) {
	r, mock := mcpMockDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `crashes`.*FOR UPDATE").WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()
	w := mcpRequest(r, "POST", "crashes/7/resolve")
	if w.Code != 404 {
		t.Fatalf("missing: %d %s", w.Code, w.Body)
	}
}
func TestMCPResolveCrashByID(t *testing.T) {
	r, mock := mcpMockDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `crashes`.*FOR UPDATE").WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"id", "resolved"}).AddRow(7, false))
	mock.ExpectExec("UPDATE `crashes` SET `resolved`=\\?,`updated_at`=\\? WHERE `crashes`.`deleted_at` IS NULL AND `id` = \\?").WithArgs(true, sqlmock.AnyArg(), 7).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	w := mcpRequest(r, "POST", "crashes/7/resolve")
	if w.Code != 200 {
		t.Fatalf("resolve: %d %s", w.Code, w.Body)
	}
}

func TestMCPDatabaseIndexOverflowFailsExplicitly(t *testing.T) {
	r, mock := mcpMockDB(t)
	rows := sqlmock.NewRows([]string{"id"})
	for i := 1; i <= 100001; i++ {
		rows.AddRow(i)
	}
	mock.ExpectQuery("SELECT .*id.* FROM `crashes`.*LIMIT 100001").WithArgs(false).WillReturnRows(rows)
	w := mcpRequest(r, "GET", "crashes")
	if w.Code != 413 || !strings.Contains(w.Body.String(), `"index_complete":false`) || strings.Contains(w.Body.String(), `"active_ids"`) {
		t.Fatalf("overflow: %d %s", w.Code, w.Body)
	}
}

func TestMCPFeedbackDetailIncludesSaveMetadata(t *testing.T) {
	r, mock := mcpMockDB(t)
	mock.ExpectQuery("SELECT .* FROM `feedbacks` WHERE deleted_at IS NULL AND id = \\? LIMIT 1").WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"id", "message"}).AddRow(9, "feedback"))
	mock.ExpectQuery("SELECT `id`,`filename`,`size`,`hash`,`content_type` FROM `savegames` WHERE feedback_id = \\? AND deleted_at IS NULL ORDER BY created_at DESC, id DESC LIMIT 1").WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"id", "filename", "size", "hash", "content_type"}).AddRow(10, "save.dat", 100, "abcdef", "application/octet-stream"))
	w := mcpRequest(r, "GET", "feedback/9")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"filename":"save.dat"`) || !strings.Contains(w.Body.String(), `"save":`) {
		t.Fatalf("detail save: %d %s", w.Code, w.Body)
	}
}
