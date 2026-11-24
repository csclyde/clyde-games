package endpoints

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func persistentFixture(t *testing.T) (*gorm.DB, models.PortalUser) {
	t.Helper()
	db := syncTestDB(t)
	if e := models.MigratePortalAuth(db); e != nil {
		t.Fatal("migration failed")
	}
	u, e := models.BootstrapPortalAdmin(db, portalFixtureInitial, time.Now().UTC())
	if e != nil {
		t.Fatal("fixture bootstrap failed")
	}
	u, e = models.ChangePortalPassword(db, u.ID, u.Version, portalFixtureInitial, portalFixtureOwn, time.Now().UTC())
	if e != nil {
		t.Fatal("fixture password change failed")
	}
	return db, u
}

func TestPortalPersistentMySQLRollingLifetimeAndNoRevival(t *testing.T) {
	db, u := persistentFixture(t)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, token, e := models.CreatePortalSession(db, u.ID, u.Version, start)
	if e != nil {
		t.Fatal("create failed")
	}
	for _, offset := range []time.Duration{2 * time.Hour, 9 * time.Hour, 25 * time.Hour, 364 * 24 * time.Hour, 600 * 24 * time.Hour, 900 * 24 * time.Hour} {
		now := start.Add(offset)
		got, expires, _, e := models.AuthenticatePortalSessionWithExpiry(db, token, now)
		if e != nil || got.ID != u.ID {
			t.Fatal("valid durable session rejected", offset, e)
		}
		if offset >= 25*time.Hour && !expires.Equal(now.Add(models.PortalSessionLifetime)) {
			t.Fatal("rolling expiry not extended", offset)
		}
	}
	var stored models.PortalSession
	if e := db.First(&stored, "token_hash = ?", models.PortalTokenHash(token)).Error; e != nil {
		t.Fatal("session missing")
	}
	if !stored.CreatedAt.Equal(start) || stored.TokenHash == token || len(stored.TokenHash) != 64 {
		t.Fatal("creation time changed or raw token stored")
	}
	// A fresh GORM/connection context still uses the persisted session, without
	// an in-memory access token or a second login.
	pool, _ := db.DB()
	pool.SetMaxIdleConns(0)
	if _, e := models.AuthenticatePortalSession(db.Session(&gorm.Session{NewDB: true}), token, start.Add(901*24*time.Hour)); e != nil {
		t.Fatal("new connection lost session")
	}
	pool.SetMaxIdleConns(2)
	if e := db.First(&stored, "token_hash = ?", models.PortalTokenHash(token)).Error; e != nil {
		t.Fatal("persisted session missing")
	}
	if _, e := models.AuthenticatePortalSession(db, token, stored.ExpiresAt); e == nil {
		t.Fatal("expired session revived")
	}
	// Legacy eight-hour sessions upgrade only while still valid.
	_, legacy, e := models.CreatePortalSession(db, u.ID, u.Version, start)
	if e != nil {
		t.Fatal("legacy fixture create")
	}
	db.Model(&models.PortalSession{}).Where("token_hash = ?", models.PortalTokenHash(legacy)).UpdateColumn("expires_at", start.Add(8*time.Hour))
	db.Model(&models.PortalSession{}).Where("token_hash = ?", models.PortalTokenHash(legacy)).UpdateColumn("last_seen_at", start.Add(7*time.Hour-time.Minute))
	_, expires, _, e := models.AuthenticatePortalSessionWithExpiry(db, legacy, start.Add(7*time.Hour))
	if e != nil || !expires.Equal(start.Add(7*time.Hour).Add(models.PortalSessionLifetime)) {
		t.Fatal("valid legacy session was not upgraded")
	}
	db.Model(&models.PortalSession{}).Where("token_hash = ?", models.PortalTokenHash(legacy)).UpdateColumn("expires_at", start.Add(8*time.Hour))
	if _, e := models.AuthenticatePortalSession(db, legacy, start.Add(9*time.Hour)); e == nil {
		t.Fatal("expired legacy session revived")
	}
	_, idleLegacy, e := models.CreatePortalSession(db, u.ID, u.Version, start)
	if e != nil {
		t.Fatal("idle legacy fixture create")
	}
	db.Model(&models.PortalSession{}).Where("token_hash = ?", models.PortalTokenHash(idleLegacy)).UpdateColumn("expires_at", start.Add(8*time.Hour))
	if _, e := models.AuthenticatePortalSession(db, idleLegacy, start.Add(30*time.Minute)); e == nil {
		t.Fatal("old idle-expired session was revived")
	}
	if _, e := models.AuthenticatePortalSession(db, idleLegacy, start.Add(31*time.Minute)); e == nil {
		t.Fatal("old idle-expired session was renewed")
	}
	if e := models.LogoutPortalSession(db, token, u.ID, start.Add(902*24*time.Hour)); e != nil {
		t.Fatal("logout failed")
	}
	if _, e := models.AuthenticatePortalSession(db, token, start.Add(902*24*time.Hour)); e == nil {
		t.Fatal("logged-out durable session accepted")
	}
}

func TestPortalPersistentMySQLCookieLifetimeAndRevocation(t *testing.T) {
	db, u := persistentFixture(t)
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("p", 32))
	t.Setenv("PORTAL_AUTH_MODE", "session")
	r := gin.New()
	RegisterPortalAuthRoutes(r)
	w := portalFixtureRequest(r, "POST", "/auth/login", map[string]string{"username": "admin", "password": portalFixtureOwn}, nil, "", 51)
	a := portalExpect(t, w, 200)
	cookie := portalResponseCookie(t, w)
	max := int(models.PortalSessionLifetime.Seconds())
	if cookie.MaxAge < max-2 || cookie.MaxAge > max || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" {
		t.Fatal("persistent cookie lifetime/protection incorrect")
	}
	if len(w.Result().Cookies()) != 1 {
		t.Fatal("ambiguous login cookies")
	}
	w = portalFixtureRequest(r, "GET", "/auth/session", nil, cookie, "", 52)
	portalExpect(t, w, 200)
	var meta struct {
		Idle       int       `json:"idle_seconds"`
		Absolute   int       `json:"absolute_seconds"`
		Persistent int       `json:"persistent_seconds"`
		Renewal    int       `json:"renewal_seconds"`
		Expires    time.Time `json:"expires_at"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &meta); e != nil || meta.Idle != 0 || meta.Absolute != 0 || meta.Persistent != max || meta.Renewal != 86400 || meta.Expires.Before(time.Now().Add(364*24*time.Hour)) {
		t.Fatal("lifetime metadata inaccurate")
	}
	if w.Header().Get("Set-Cookie") != "" {
		t.Fatal("cookie unnecessarily reissued before daily renewal")
	}
	// A still-valid eight-hour session from the old policy upgrades in place.
	db.Model(&models.PortalSession{}).Where("token_hash = ?", models.PortalTokenHash(cookie.Value)).UpdateColumn("expires_at", time.Now().UTC().Add(8*time.Hour))
	w = portalFixtureRequest(r, "GET", "/auth/session", nil, cookie, "", 52)
	portalExpect(t, w, 200)
	renewed := portalResponseCookie(t, w)
	if renewed.Value != cookie.Value || renewed.MaxAge < max-2 {
		t.Fatal("legacy cookie upgrade failed or unexpectedly rotated CSRF")
	}

	if e := models.RevokePortalUserSessions(db, u.ID, u.Version, u.ID, u.Version, time.Now().UTC()); e != nil {
		t.Fatal("administrator revocation failed")
	}
	w = portalFixtureRequest(r, "GET", "/auth/session", nil, cookie, "", 53)
	portalExpect(t, w, 401)
	if w.Header().Get("Set-Cookie") != "" {
		t.Fatal("stale failure can delete a newer cookie")
	}
	// Explicit logout still clears both browser and server state.
	w = portalFixtureRequest(r, "POST", "/auth/login", map[string]string{"username": "admin", "password": portalFixtureOwn}, nil, "", 54)
	a = portalExpect(t, w, 200)
	cookie = portalResponseCookie(t, w)
	w = portalFixtureRequest(r, "POST", "/auth/logout", map[string]string{}, cookie, a.CSRF, 55)
	portalExpect(t, w, 200)
	if len(w.Result().Cookies()) != 1 || portalResponseCookie(t, w).MaxAge != -1 {
		t.Fatal("logout cookie not unambiguously cleared")
	}
	portalExpect(t, portalFixtureRequest(r, "GET", "/auth/session", nil, cookie, "", 56), 401)
	// Temporary credentials retain eight-hour expiry with mandatory-change gates.
	fresh, e := models.PortalFindUser(db, "admin")
	if e != nil {
		t.Fatal("admin read")
	}
	if e := models.RecoverPortalAdmin(db, "admin", portalFixtureInitial, time.Now().UTC()); e != nil {
		t.Fatal("temporary fixture reset")
	}
	w = portalFixtureRequest(r, "POST", "/auth/login", map[string]string{"username": "admin", "password": portalFixtureInitial}, nil, "", 57)
	a = portalExpect(t, w, 200)
	cookie = portalResponseCookie(t, w)
	if !a.User.MustChangePassword || cookie.MaxAge > int(models.PortalTemporarySessionLifetime.Seconds()) || a.User.Version <= fresh.Version {
		t.Fatal("temporary session protections weakened")
	}
	var temporary models.PortalSession
	db.First(&temporary, "token_hash = ?", models.PortalTokenHash(cookie.Value))
	if _, e := models.AuthenticatePortalSession(db, cookie.Value, temporary.ExpiresAt); e == nil {
		t.Fatal("temporary session renewed beyond eight hours")
	}
}

func TestPortalPersistentMySQLConcurrentTouchDoesNotSignOut(t *testing.T) {
	db, u := persistentFixture(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	_, token, e := models.CreatePortalSession(db, u.ID, u.Version, now.Add(-2*time.Minute))
	if e != nil {
		t.Fatal("create")
	}
	arrived := make(chan struct{}, 2)
	release := make(chan struct{})
	var reads int32
	name := "persistent-overlapping-reads"
	db.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "portal_sessions" && tx.Error == nil && atomic.AddInt32(&reads, 1) <= 2 {
			arrived <- struct{}{}
			<-release
		}
	})
	defer db.Callback().Query().Remove(name)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := models.AuthenticatePortalSession(db, token, now); errs <- e }()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-arrived:
		case <-time.After(5 * time.Second):
			close(release)
			wg.Wait()
			t.Fatal("concurrent read barrier not reached")
		}
	}
	close(release)
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal("same-millisecond activity falsely signed out", e)
		}
	}
}

func TestPortalPersistentMySQLLogoutDuringRenewalCannotResurrect(t *testing.T) {
	db, u := persistentFixture(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	_, token, e := models.CreatePortalSession(db, u.ID, u.Version, now.Add(-25*time.Hour))
	if e != nil {
		t.Fatal("create")
	}
	updated := make(chan struct{}, 1)
	release := make(chan struct{})
	var touched int32
	name := "persistent-renewal-committed"
	db.Callback().Update().After("gorm:commit_or_rollback_transaction").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "portal_sessions" && tx.Error == nil && atomic.AddInt32(&touched, 1) == 1 {
			updated <- struct{}{}
			<-release
		}
	})
	defer db.Callback().Update().Remove(name)
	done := make(chan error, 1)
	go func() { _, e := models.AuthenticatePortalSession(db, token, now); done <- e }()
	select {
	case <-updated:
	case <-time.After(5 * time.Second):
		close(release)
		<-done
		t.Fatal("renewal barrier not reached")
	}
	e = models.LogoutPortalSession(db, token, u.ID, now)
	close(release)
	if e != nil {
		t.Fatal("logout failed")
	}
	select {
	case e = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("renewal did not complete")
	}
	if e == nil || (!errors.Is(e, gorm.ErrRecordNotFound) && !errors.Is(e, models.ErrPortalAuth)) {
		t.Fatal("revoked session accepted after renewal")
	}
	var count int64
	db.Model(&models.PortalSession{}).Where("token_hash = ?", models.PortalTokenHash(token)).Count(&count)
	if count != 0 {
		t.Fatal("renewal recreated logged-out session")
	}
}
