package endpoints

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

const portalFixtureInitial = "initial fixture passphrase only"
const portalFixtureOwn = "changed fixture passphrase only"

type portalReply struct {
	User     models.PortalUser   `json:"user"`
	CSRF     string              `json:"csrf"`
	Password string              `json:"temporary_password"`
	Items    []models.PortalUser `json:"items"`
}

func portalFixtureRequest(r *gin.Engine, method, path string, body any, cookie *http.Cookie, csrf string, ip int) *httptest.ResponseRecorder {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Host = "admin.clyde.games"
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Admin-API-Token", strings.Repeat("p", 32))
	req.Header.Set("X-Clyde-Portal", "1")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Real-IP", fmt.Sprintf("203.0.113.%d", ip))
	if method != "GET" {
		req.Header.Set("Origin", portalOrigin)
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func portalExpect(t *testing.T, w *httptest.ResponseRecorder, status int) portalReply {
	t.Helper()
	if w.Code != status {
		t.Fatalf("unexpected HTTP status: got%d want%d (credential-bearing body intentionally omitted)", w.Code, status)
	}
	var data portalReply
	if status == 200 {
		if json.Unmarshal(w.Body.Bytes(), &data) != nil {
			t.Fatal("invalid response JSON")
		}
	}
	return data
}
func portalResponseCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == portalCookie {
			return c
		}
	}
	t.Fatal("session cookie missing")
	return nil
}
func TestPortalAuthGatewayAndCompatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("p", 32))
	t.Setenv("PORTAL_AUTH_MODE", "session")
	r := gin.New()
	RegisterPortalAuthRoutes(r)
	r.GET("/private", AdminAuth, func(c *gin.Context) { c.Status(204) })
	// A token injected into a browser proxy request is never session authentication.
	portalExpect(t, portalFixtureRequest(r, "GET", "/private", nil, nil, "", 1), 401)
	for _, path := range []string{"/auth/session", "/auth/users", "/auth/signup", "/auth/bootstrap"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 401 && w.Code != 404 {
			t.Fatal("unauthorized route accessible")
		}
	}
	req := httptest.NewRequest("GET", "/private", nil)
	req.Header.Set("X-Admin-API-Token", strings.Repeat("p", 32))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatal("machine token compatibility changed")
	}
	t.Setenv("PORTAL_AUTH_MODE", "legacy")
	portalExpect(t, portalFixtureRequest(r, "GET", "/auth/session", nil, nil, "", 2), 200)
	portalExpect(t, portalFixtureRequest(r, "POST", "/auth/login", map[string]string{}, nil, "", 3), 404)
	t.Setenv("PORTAL_AUTH_MODE", "typo")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatal("invalid configuration must fail closed")
	}
}
func TestPortalLoginRateLimitAndSafePanicLogging(t *testing.T) {
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("p", 32))
	t.Setenv("PORTAL_AUTH_MODE", "session")
	portalLimits.Lock()
	portalLimits.Items = map[string]portalLimitEntry{}
	portalLimits.Unlock()
	r := gin.New()
	RegisterPortalAuthRoutes(r)
	for i := 0; i < 10; i++ {
		portalExpect(t, portalFixtureRequest(r, "POST", "/auth/login", map[string]string{"username": "invalid!", "password": portalFixtureInitial}, nil, "", 90), 401)
	}
	portalExpect(t, portalFixtureRequest(r, "POST", "/auth/login", map[string]string{"username": "invalid!", "password": portalFixtureInitial}, nil, "", 90), 429)
	// The invalid-name path does uniform work without ever treating it as a real username.
	var out bytes.Buffer
	old := log.Writer()
	log.SetOutput(&out)
	defer log.SetOutput(old)
	panicRouter := gin.New()
	panicRouter.Use(SafePortalRequestLog(), SafePortalRecovery())
	panicRouter.GET("/panic", func(c *gin.Context) { panic("never-log-fixture-credential") })
	req := httptest.NewRequest("GET", "/panic?password=never-log-fixture-credential", nil)
	req.Header.Set("Cookie", "session=never-log-fixture-credential")
	w := httptest.NewRecorder()
	panicRouter.ServeHTTP(w, req)
	if w.Code != 500 || strings.Contains(out.String(), "never-log-fixture-credential") {
		t.Fatal("panic/log sanitation failed")
	}
}
func TestPortalBrowserBoundaryRejectsBeforeDatabase(t *testing.T) {
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("p", 32))
	t.Setenv("PORTAL_AUTH_MODE", "session")
	r := gin.New()
	RegisterPortalAuthRoutes(r)
	for _, modify := range []func(*http.Request){
		func(r *http.Request) { r.Header.Del("Origin") },
		func(r *http.Request) { r.Header.Set("Origin", "https://other.example") },
		func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") },
		func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "http") },
		func(r *http.Request) { r.Header.Del("X-Clyde-Portal") },
		func(r *http.Request) { r.Host = "api.clyde.games" },
	} {
		req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{"username":"admin","password":"fixture passphrase only"}`))
		req.Host = "admin.clyde.games"
		req.Header.Set("Origin", portalOrigin)
		req.Header.Set("X-Admin-API-Token", strings.Repeat("p", 32))
		req.Header.Set("X-Clyde-Portal", "1")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("Content-Type", "application/json")
		modify(req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 401 && w.Code != 403 {
			t.Fatal("untrusted browser gateway reached login")
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("auth response cacheable")
		}
	}
	for _, cookie := range []string{"", portalCookie + "=short", portalCookie + "=one; " + portalCookie + "=two"} {
		req := httptest.NewRequest("GET", "/auth/session", nil)
		req.Host = "admin.clyde.games"
		req.Header.Set("X-Admin-API-Token", strings.Repeat("p", 32))
		req.Header.Set("X-Clyde-Portal", "1")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("Cookie", cookie)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatal("invalid cookie accepted")
		}
	}
}
func TestPortalAuthMySQLLifecycle(t *testing.T) {
	db := syncTestDB(t)
	for i := 0; i < 2; i++ {
		if models.MigratePortalAuth(db) != nil {
			t.Fatal("migration failed")
		}
	}
	var count int64
	db.Model(&models.PortalUser{}).Count(&count)
	if count != 0 {
		t.Fatal("migration created credentials")
	}
	admin, e := models.BootstrapPortalAdmin(db, portalFixtureInitial, time.Now().UTC())
	if e != nil {
		t.Fatal("test bootstrap failed")
	}
	if _, e = models.BootstrapPortalAdmin(db, portalFixtureInitial, time.Now().UTC()); e != models.ErrPortalConflict {
		t.Fatal("bootstrap repeated")
	}
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("p", 32))
	t.Setenv("PORTAL_AUTH_MODE", "session")
	r := gin.New()
	RegisterPortalAuthRoutes(r)
	writes := 0
	r.GET("/private", AdminAuth, func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	r.POST("/private", AdminAuth, func(c *gin.Context) { writes++; c.JSON(200, gin.H{"ok": true}) })
	r.POST("/rebuild", PortalPrivilegedBridge, func(c *gin.Context) { writes++; c.JSON(200, gin.H{"ok": true}) })
	ip := 100
	request := func(method, path string, body any, c *http.Cookie, csrf string) *httptest.ResponseRecorder {
		ip++
		return portalFixtureRequest(r, method, path, body, c, csrf, ip)
	}
	wrong := request("POST", "/auth/login", map[string]string{"username": "admin", "password": "wrong fixture password"}, nil, "")
	portalExpect(t, wrong, 401)
	missing := request("POST", "/auth/login", map[string]string{"username": "nonexistent", "password": "wrong fixture password"}, nil, "")
	portalExpect(t, missing, 401)
	if wrong.Body.String() != missing.Body.String() {
		t.Fatal("account enumeration response differs")
	}
	w := request("POST", "/auth/login", map[string]string{"username": "admin", "password": portalFixtureInitial}, nil, "")
	a := portalExpect(t, w, 200)
	cookie := portalResponseCookie(t, w)
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Domain != "" || cookie.Path != "/" || len(cookie.Value) != 43 {
		t.Fatal("cookie protection flags")
	}
	if strings.Contains(w.Body.String(), cookie.Value) || strings.Contains(w.Body.String(), portalFixtureInitial) || strings.Contains(w.Body.String(), "password_hash") {
		t.Fatal("session/password material leaked")
	}
	if !a.User.MustChangePassword {
		t.Fatal("temporary password not gated")
	}
	portalExpect(t, request("GET", "/private", nil, cookie, ""), 403)
	portalExpect(t, request("POST", "/auth/password", map[string]string{"current_password": portalFixtureInitial, "new_password": portalFixtureOwn}, cookie, ""), 403)
	w = request("POST", "/auth/password", map[string]string{"current_password": portalFixtureInitial, "new_password": portalFixtureOwn}, cookie, a.CSRF)
	a = portalExpect(t, w, 200)
	oldCookie := cookie
	cookie = portalResponseCookie(t, w)
	if oldCookie.Value == cookie.Value || a.User.MustChangePassword {
		t.Fatal("password change did not rotate session")
	}
	portalExpect(t, request("GET", "/private", nil, oldCookie, ""), 401)
	portalExpect(t, request("GET", "/private", nil, cookie, ""), 200)
	portalExpect(t, request("POST", "/private", map[string]string{}, cookie, ""), 403)
	portalExpect(t, request("POST", "/private", map[string]string{}, cookie, a.CSRF), 200)
	portalExpect(t, request("POST", "/private", map[string]string{}, cookie, "forged-csrf"), 403)
	portalExpect(t, request("POST", "/auth/users", map[string]any{"username": "unauthorized", "role": "viewer"}, cookie, "wrong-csrf"), 403)
	portalExpect(t, request("POST", "/auth/users", map[string]any{"username": "invalid-role", "role": "owner"}, cookie, a.CSRF), 400)
	unchanged := request("PATCH", fmt.Sprintf("/auth/users/%d", admin.ID), map[string]any{"version": a.User.Version, "role": "admin"}, cookie, a.CSRF)
	portalExpect(t, unchanged, 200)
	if !strings.Contains(unchanged.Body.String(), `"sessions_revoked":false`) {
		t.Fatal("no-op falsely claimed revocation")
	}
	portalExpect(t, request("GET", "/private", nil, cookie, ""), 200)
	portalExpect(t, request("PATCH", fmt.Sprintf("/auth/users/%d", admin.ID), map[string]any{"version": a.User.Version, "disabled": true}, cookie, a.CSRF), 409)
	w = request("POST", "/auth/users", map[string]any{"username": "reviewer", "role": "viewer"}, cookie, a.CSRF)
	created := portalExpect(t, w, 200)
	if created.Password == "" || created.User.Role != "viewer" {
		t.Fatal("user creation failed")
	}
	portalExpect(t, request("POST", "/auth/users", map[string]any{"username": "reviewer", "role": "viewer"}, cookie, a.CSRF), 409)
	portalExpect(t, request("PATCH", fmt.Sprintf("/auth/users/%d", created.User.ID), map[string]any{"version": created.User.Version + 1, "disabled": true}, cookie, a.CSRF), 409)
	// A known session ID is never adopted on a later login; same-user predecessor is revoked.
	w = request("POST", "/auth/login", map[string]string{"username": "admin", "password": portalFixtureOwn}, cookie, "")
	a = portalExpect(t, w, 200)
	predecessor := cookie
	cookie = portalResponseCookie(t, w)
	if predecessor.Value == cookie.Value {
		t.Fatal("session fixation")
	}
	portalExpect(t, request("GET", "/private", nil, predecessor, ""), 401)
	w = request("POST", "/auth/login", map[string]string{"username": "reviewer", "password": created.Password}, nil, "")
	viewer := portalExpect(t, w, 200)
	vc := portalResponseCookie(t, w)
	w = request("POST", "/auth/password", map[string]string{"current_password": created.Password, "new_password": "viewer changed fixture passphrase"}, vc, viewer.CSRF)
	viewer = portalExpect(t, w, 200)
	vc = portalResponseCookie(t, w)
	portalExpect(t, request("GET", "/private", nil, vc, ""), 200)
	beforeWrites := writes
	portalExpect(t, request("POST", "/private", map[string]string{}, vc, viewer.CSRF), 403)
	portalExpect(t, request("POST", "/rebuild", map[string]string{}, vc, viewer.CSRF), 403)
	if writes != beforeWrites {
		t.Fatal("viewer mutation executed")
	}
	portalExpect(t, request("GET", "/auth/users", nil, vc, ""), 403)
	portalExpect(t, request("POST", "/auth/users", map[string]string{}, vc, viewer.CSRF), 403)
	portalExpect(t, request("POST", fmt.Sprintf("/auth/users/%d/revoke", viewer.User.ID), map[string]any{"version": viewer.User.Version}, cookie, a.CSRF), 200)
	portalExpect(t, request("GET", "/private", nil, vc, ""), 401)
	var fresh models.PortalUser
	db.First(&fresh, viewer.User.ID)
	w = request("PATCH", fmt.Sprintf("/auth/users/%d", fresh.ID), map[string]any{"version": fresh.Version, "disabled": true}, cookie, a.CSRF)
	disabled := portalExpect(t, w, 200).User
	portalExpect(t, request("POST", "/auth/login", map[string]string{"username": "reviewer", "password": "viewer changed fixture passphrase"}, nil, ""), 401)
	w = request("PATCH", fmt.Sprintf("/auth/users/%d", fresh.ID), map[string]any{"version": disabled.Version, "disabled": false}, cookie, a.CSRF)
	enabled := portalExpect(t, w, 200).User
	w = request("POST", fmt.Sprintf("/auth/users/%d/reset-password", fresh.ID), map[string]any{"version": enabled.Version}, cookie, a.CSRF)
	reset := portalExpect(t, w, 200)
	if !reset.User.MustChangePassword || reset.Password == "" {
		t.Fatal("password reset failed")
	}
	portalExpect(t, request("POST", "/auth/login", map[string]string{"username": "reviewer", "password": "viewer changed fixture passphrase"}, nil, ""), 401)
	w = request("POST", "/auth/login", map[string]string{"username": "reviewer", "password": reset.Password}, nil, "")
	portalExpect(t, w, 200)
	expiredCookie := portalResponseCookie(t, w)
	db.Model(&models.PortalSession{}).Where("token_hash = ?", models.PortalTokenHash(expiredCookie.Value)).UpdateColumn("last_seen_at", time.Now().UTC().Add(-2*time.Hour))
	// Thirty-minute idle expiry is gone, including on a temporary session.
	portalExpect(t, request("GET", "/auth/session", nil, expiredCookie, ""), 200)
	db.Model(&models.PortalSession{}).Where("token_hash = ?", models.PortalTokenHash(expiredCookie.Value)).UpdateColumn("expires_at", time.Now().UTC().Add(-time.Second))
	w = request("GET", "/auth/session", nil, expiredCookie, "")
	portalExpect(t, w, 401)
	if w.Header().Get("Set-Cookie") != "" {
		t.Fatal("stale unauthorized response can erase a newer login cookie")
	}
	_, expired, e := models.CreatePortalSession(db, admin.ID, a.User.Version, time.Now().UTC().Add(-models.PortalSessionLifetime-time.Minute))
	if e != nil {
		t.Fatal("expiry setup")
	}
	db.Model(&models.PortalSession{}).Where("token_hash = ?", models.PortalTokenHash(expired)).UpdateColumn("last_seen_at", time.Now().UTC())
	portalExpect(t, request("GET", "/auth/session", nil, &http.Cookie{Name: portalCookie, Value: expired}, ""), 401)
	portalExpect(t, request("POST", "/auth/logout", map[string]string{}, cookie, ""), 403)
	w = request("POST", "/auth/logout", map[string]string{}, cookie, a.CSRF)
	portalExpect(t, w, 200)
	if portalResponseCookie(t, w).MaxAge != -1 {
		t.Fatal("logout did not clear cookie")
	}
	portalExpect(t, request("GET", "/private", nil, cookie, ""), 401)
	if models.RecoverPortalAdmin(db, "admin", portalFixtureInitial, time.Now().UTC()) != nil {
		t.Fatal("operator recovery failed")
	}
	recovered, e := models.PortalFindUser(db, "admin")
	if e != nil || !recovered.MustChangePassword || recovered.Disabled {
		t.Fatal("recovery state")
	}
	db.Model(&models.PortalSession{}).Where("user_id = ?", admin.ID).Count(&count)
	if count != 0 {
		t.Fatal("recovery retained sessions")
	}
}
func TestPortalLastAdministratorMySQLConcurrency(t *testing.T) {
	db := syncTestDB(t)
	if models.MigratePortalAuth(db) != nil {
		t.Fatal("migration")
	}
	a, e := models.BootstrapPortalAdmin(db, portalFixtureInitial, time.Now().UTC())
	if e != nil {
		t.Fatal("bootstrap")
	}
	a, e = models.ChangePortalPassword(db, a.ID, a.Version, portalFixtureInitial, portalFixtureOwn, time.Now().UTC())
	if e != nil {
		t.Fatal("password")
	}
	b, e := models.CreatePortalUser(db, a.ID, a.Version, "second-admin", "admin", portalFixtureInitial, time.Now().UTC())
	if e != nil {
		t.Fatal("create")
	}
	b, e = models.ChangePortalPassword(db, b.ID, b.Version, portalFixtureInitial, portalFixtureOwn, time.Now().UTC())
	if e != nil {
		t.Fatal("password")
	}
	role := "viewer"
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, u := range []models.PortalUser{a, b} {
		wg.Add(1)
		go func(u models.PortalUser) {
			defer wg.Done()
			_, e := models.UpdatePortalUser(db, u.ID, u.Version, u.ID, u.Version, &role, nil, time.Now().UTC())
			results <- e
		}(u)
	}
	wg.Wait()
	close(results)
	ok, blocked := 0, 0
	for e := range results {
		if e == nil {
			ok++
		} else if e == models.ErrPortalLastAdmin {
			blocked++
		} else {
			t.Fatal("concurrent update failed unexpectedly")
		}
	}
	if ok != 1 || blocked != 1 {
		t.Fatal("last-admin invariant")
	}
	var count int64
	db.Model(&models.PortalUser{}).Where("role = ? AND disabled = ?", "admin", false).Count(&count)
	if count != 1 {
		t.Fatal("last admin lost")
	}
}
