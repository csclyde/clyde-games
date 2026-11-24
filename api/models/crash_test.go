package models

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
)

func TestCrashIdentity(t *testing.T) {
	base := Crash{Project: "cursemark", Message: "Null object reference", Category: "error", Platform: "windows", Stack: "FilePos(Method(game.Player,update),game/Player.hx,25,8), FilePos(Method(game.Main,tick),game/Main.hx,40,2)\nGPU Model: A\nScene: one"}
	key, identity := CrashIdentityV1(base)
	if !strings.HasPrefix(key, "v1:") || len(key) != 67 || strings.Contains(identity, "GPU") {
		t.Fatalf("invalid identity: %s %s", key, identity)
	}
	for name, mutate := range map[string]func(*Crash){
		"different runtime metadata": func(c *Crash) {
			c.PID = "other"
			c.Env = "different"
			c.Build = "2026-11-16"
			c.Commit = "other"
			c.Stack = strings.ReplaceAll(c.Stack, "GPU Model: A\nScene: one", "GPU Model: B\nScene: two")
		},
		"near stack with shifted caller source lines": func(c *Crash) {
			c.Stack = strings.ReplaceAll(c.Stack, ",40,2", ",42,4")
		},
		"Haxe array and multiline formatting": func(c *Crash) { c.Stack = "[" + strings.ReplaceAll(c.Stack, "), FilePos", "),\nFilePos") + "]" },
		"line endings":                        func(c *Crash) { c.Stack = strings.ReplaceAll(c.Stack, "\n", "\r\n") },
	} {
		t.Run(name, func(t *testing.T) {
			c := base
			mutate(&c)
			got, _ := CrashIdentityV1(c)
			if got != key {
				t.Fatalf("equivalent reports differ: %s", got)
			}
		})
	}
	for name, mutate := range map[string]func(*Crash){
		"project":                              func(c *Crash) { c.Project = "necrovale" },
		"error cause":                          func(c *Crash) { c.Message = "Index out of bounds" },
		"platform":                             func(c *Crash) { c.Platform = "linux" },
		"category":                             func(c *Crash) { c.Category = "assertion" },
		"top frame":                            func(c *Crash) { c.Stack = strings.ReplaceAll(c.Stack, "Player,update", "Enemy,update") },
		"shared top frame different caller":    func(c *Crash) { c.Stack = strings.ReplaceAll(c.Stack, "Main,tick", "Menu,tick") },
		"different file":                       func(c *Crash) { c.Stack = strings.ReplaceAll(c.Stack, "game/Player.hx", "ui/Player.hx") },
		"unknown added frame":                  func(c *Crash) { c.Stack = "unknown\n" + c.Stack },
		"different failure line same function": func(c *Crash) { c.Stack = strings.ReplaceAll(c.Stack, ",25,8", ",29,8") },
	} {
		t.Run(name, func(t *testing.T) {
			c := base
			mutate(&c)
			got, _ := CrashIdentityV1(c)
			if got == key {
				t.Fatal("unrelated cause merged")
			}
		})
	}
	for _, tc := range []struct {
		a, b Crash
		same bool
	}{
		{Crash{Message: "Access Violation reading 0x1234", Stack: "0x1234 game!Player.update+0x12\nMain.cpp:22"}, Crash{Message: "Access Violation reading 0x9876", Stack: "0x5678 game!Player.update+0x12\nMain.cpp:23"}, true},
		{Crash{Message: "Access Violation reading 0x1234", Stack: "game!Player.update+0x12"}, Crash{Message: "Access Violation reading 0x1234", Stack: "game!Player.update+0x56"}, false},
		{Crash{Message: "Access Violation reading 0x1234", Stack: "0x1111"}, Crash{Message: "Access Violation reading 0x5678", Stack: "0x2222"}, false},
		{Crash{Message: "error 42", Stack: "Player.update"}, Crash{Message: "error 43", Stack: "Player.update"}, false},
		{Crash{Message: "ab", Stack: "c"}, Crash{Message: "a", Stack: "bc"}, false},
	} {
		a, _ := CrashIdentityV1(tc.a)
		b, _ := CrashIdentityV1(tc.b)
		if (a == b) != tc.same {
			t.Errorf("identity equality got %v want %v", a == b, tc.same)
		}
	}
}

func crashDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	db, mock := cutoffMock(t)
	old := AnalyticsDB
	AnalyticsDB = db
	t.Cleanup(func() { AnalyticsDB = old })
	return mock
}

func expectCrashLock(mock sqlmock.Sqlmock, key string) {
	mock.ExpectExec("INSERT INTO `crash_groups`.*ON DUPLICATE KEY UPDATE").WithArgs(key).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM `crash_groups`.*FOR UPDATE").WithArgs(key).WillReturnRows(sqlmock.NewRows([]string{"key"}).AddRow(key))
}

func TestAddCrashAlwaysInsertsCompleteIndependentOccurrence(t *testing.T) {
	mock := crashDB(t)
	input := Crash{Model: gorm.Model{ID: 999, CreatedAt: time.Unix(1, 0), DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}, Project: "Cursemark", PID: "first", Message: "failure", Stack: "Player.hx:10\nGPU Model: A", Env: "prod", Category: "error", Platform: "windows", Build: "2026-11-15", Commit: "one", Hash: "oldhash", Count: 300, Resolved: true, GroupKey: "forged", OccurrenceVersion: 99}
	var previous *Crash
	for i := 1; i <= 2; i++ {
		if i == 2 {
			input.PID = "second"
			input.Stack = "Player.hx:10\nGPU Model: B"
			input.Env = "test"
			input.Build = "2026-11-16"
			input.Commit = "two"
		}
		key, identity := CrashIdentityV1(input)
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO `projects`").WithArgs(false, "cursemark", "cursemark", "").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT .* FROM `event_settings`").WithArgs(projectCrashSettingKey("cursemark")).WillReturnRows(sqlmock.NewRows([]string{"key", "value"}))
		expectCrashLock(mock, key)
		// Assert every submitted metadata field, a fresh timestamp/ID and no
		// client-supplied count, deletion, group or resolution state reaches SQL.
		mock.ExpectExec("INSERT INTO `crashes`").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), nil, input.PID, "cursemark", input.Message, input.Stack, input.Env, input.Category, input.Platform, input.Build, input.Commit, input.Hash, 1, false, 1, sqlmock.AnyArg(), nil, key, identity).WillReturnResult(sqlmock.NewResult(int64(i), 1))
		mock.ExpectCommit()
		got, err := AddCrash(input)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != uint(i) || got.Resolved || got.Count != 1 || got.IngestedAt == nil || got.DeletedAt.Valid || got.OccurrenceVersion != 1 || !got.CreatedAt.Equal(*got.IngestedAt) {
			t.Fatalf("bad stored occurrence: %+v", got)
		}
		if previous != nil && (got.ID == previous.ID || got.GroupKey != previous.GroupKey || got.PID == previous.PID || got.Stack == previous.Stack || got.Build == previous.Build || got.Commit == previous.Commit) {
			t.Fatal("lost per-occurrence metadata or grouping")
		}
		previous = got
	}
}

func TestResolveCrashGroupRepeatedAndImmutableIngestion(t *testing.T) {
	mock := crashDB(t)
	created, resolved := time.Unix(100, 0), time.Unix(200, 0)
	for _, changed := range []int64{3, 0} {
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT .* FROM `crashes`.*LIMIT 1$").WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"id", "group_key", "created_at", "updated_at"}).AddRow(7, "v1:group", created, created))
		expectCrashLock(mock, "v1:group")
		// All members, no ID-only predicate, no UpdatedAt or Count update. A
		// repeated resolve excludes previously resolved members/timestamps.
		mock.ExpectExec("UPDATE `crashes` SET `resolved`=\\?,`resolved_at`=\\? WHERE \\(group_key = \\? AND resolved = \\?\\) AND `crashes`.`deleted_at` IS NULL").WithArgs(true, sqlmock.AnyArg(), "v1:group", false).WillReturnResult(sqlmock.NewResult(0, changed))
		mock.ExpectQuery("SELECT .* FROM `crashes`.*FOR UPDATE").WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"id", "group_key", "resolved", "created_at", "updated_at", "ingested_at", "resolved_at"}).AddRow(7, "v1:group", true, created, created, created, resolved))
		mock.ExpectCommit()
		got, err := ResolveCrashID(AnalyticsDB, 7)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Resolved || !got.CreatedAt.Equal(created) || !got.UpdatedAt.Equal(created) || !got.IngestedAt.Equal(created) || !got.ResolvedAt.Equal(resolved) {
			t.Fatal("changed occurrence/resolution timestamps")
		}
	}
}

func TestCrashResolutionErrorsDoNotSucceed(t *testing.T) {
	mock := crashDB(t)
	mock.ExpectQuery("SELECT .* FROM `crashes`").WithArgs("collision").WillReturnRows(sqlmock.NewRows([]string{"id", "group_key"}).AddRow(1, "a").AddRow(2, "b"))
	if _, err := ResolveCrash("collision"); err == nil {
		t.Fatal("ambiguous hash closed groups")
	}
	mock.ExpectQuery("SELECT .* FROM `crashes`").WithArgs("hash").WillReturnError(errors.New("query failed"))
	if _, err := ResolveCrash("hash"); err == nil {
		t.Fatal("query error swallowed")
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `crashes`").WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"id", "group_key"}).AddRow(7, "group"))
	expectCrashLock(mock, "group")
	mock.ExpectExec("UPDATE `crashes`").WillReturnError(errors.New("write failed"))
	mock.ExpectRollback()
	if _, err := ResolveCrashID(AnalyticsDB, 7); err == nil {
		t.Fatal("resolution write error swallowed")
	}
}

func TestCrashMigrationPreservesLegacyHistoryAndResumes(t *testing.T) {
	mock := crashDB(t)
	legacy := Crash{Project: "cursemark", Message: "failure", Stack: "trace"}
	key, identity := CrashIdentityV1(legacy)
	mock.ExpectQuery("SELECT .* FROM `crashes` WHERE group_key IS NULL OR group_key = '' ORDER BY id ASC LIMIT 250").WillReturnRows(sqlmock.NewRows([]string{"id", "project", "message", "stack", "count", "resolved"}).AddRow(4, legacy.Project, legacy.Message, legacy.Stack, 42, true))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `crashes` SET `group_identity`=\\?,`group_key`=\\? WHERE id = \\? AND \\(group_key IS NULL OR group_key = ''\\)").WithArgs(identity, key, 4).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	for i := 0; i < 2; i++ {
		mock.ExpectQuery("SELECT .* FROM `crashes` WHERE group_key IS NULL OR group_key = '' ORDER BY id ASC LIMIT 250").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	}
	if err := MigrateCrashGroups(); err != nil {
		t.Fatal(err)
	}
	if err := MigrateCrashGroups(); err != nil {
		t.Fatal(err)
	}
}
