package endpoints

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

const portalOrigin = "https://admin.clyde.games"
const portalCookie = "__Host-clyde-session"
const portalUserContext = "portal-auth-user"
const portalExpiryContext = "portal-auth-expiry"

func PortalAuthMode() string {
	v := os.Getenv("PORTAL_AUTH_MODE")
	if v == "" || v == "legacy" {
		return "legacy"
	}
	if v == "session" {
		return v
	}
	return "invalid"
}

// Dedicated machine-token endpoints keep their existing credentials. Portal
// browser calls must additionally satisfy the session/role/CSRF boundary.
func PortalPrivilegedBridge(c *gin.Context) {
	if PortalAuthMode() == "invalid" {
		c.AbortWithStatusJSON(503, gin.H{"error": "authentication configuration unavailable"})
		return
	}
	if PortalAuthMode() == "session" && isPortalBrowser(c) {
		AdminAuth(c)
		return
	}
	c.Next()
}
func validAdminToken(c *gin.Context) bool {
	v := os.Getenv("ADMIN_API_TOKEN")
	return len(v) >= 32 && subtle.ConstantTimeCompare([]byte(v), []byte(c.GetHeader("X-Admin-API-Token"))) == 1
}
func isPortalBrowser(c *gin.Context) bool {
	for _, cookie := range c.Request.Cookies() {
		if cookie.Name == portalCookie {
			return true
		}
	}
	return c.Request.Host == "admin.clyde.games" || c.GetHeader("X-Clyde-Portal") != "" || c.GetHeader("Origin") != "" || c.GetHeader("Sec-Fetch-Site") != ""
}
func portalNoStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Writer.Header().Del("Access-Control-Allow-Origin")
	c.Writer.Header().Del("Access-Control-Allow-Credentials")
}
func trustedPortalGateway(c *gin.Context) bool {
	return validAdminToken(c) && c.GetHeader("X-Clyde-Portal") == "1" && c.Request.Host == "admin.clyde.games" && c.GetHeader("X-Forwarded-Proto") == "https"
}
func portalOriginOK(c *gin.Context, mutation bool) bool {
	origin := c.GetHeader("Origin")
	if c.GetHeader("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	if mutation {
		return origin == portalOrigin
	}
	return origin == "" || origin == portalOrigin
}
func portalGateway(c *gin.Context) bool {
	portalNoStore(c)
	if PortalAuthMode() != "session" {
		c.AbortWithStatusJSON(404, gin.H{"error": "not found"})
		return false
	}
	if !trustedPortalGateway(c) {
		c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
		return false
	}
	if !portalOriginOK(c, c.Request.Method != "GET" && c.Request.Method != "HEAD") {
		c.AbortWithStatusJSON(403, gin.H{"error": "cross-origin request denied"})
		return false
	}
	return true
}
func setPortalCookie(c *gin.Context, token string, expires, now time.Time) {
	// The persisted database expiry is authoritative. A positive Max-Age keeps
	// the host-only, HTTPS-only cookie across browser restarts.
	if !expires.After(now) {
		return
	}
	maxAge := int((expires.Sub(now) + time.Second - 1) / time.Second)
	http.SetCookie(c.Writer, &http.Cookie{Name: portalCookie, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: maxAge, Expires: expires.UTC()})
}
func clearPortalCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{Name: portalCookie, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0).UTC()})
}
func portalRawToken(c *gin.Context) string {
	token := ""
	count := 0
	for _, cookie := range c.Request.Cookies() {
		if cookie.Name == portalCookie {
			count++
			token = cookie.Value
		}
	}
	if count != 1 {
		return ""
	}
	return token
}
func portalAccount(c *gin.Context) (models.PortalUser, bool) {
	v, ok := c.Get(portalUserContext)
	if !ok {
		return models.PortalUser{}, false
	}
	u, ok := v.(models.PortalUser)
	return u, ok
}
func portalSessionAuth(c *gin.Context, allowPasswordChange bool) bool {
	if !portalGateway(c) {
		return false
	}
	token := portalRawToken(c)
	if len(token) != 43 {
		c.AbortWithStatusJSON(401, gin.H{"error": "sign in required"})
		return false
	}
	u, expires, renewed, e := models.AuthenticatePortalSessionWithExpiry(mcpDB(c), token, time.Now().UTC())
	if e != nil {
		if errors.Is(e, models.ErrPortalAuth) || errors.Is(e, gorm.ErrRecordNotFound) {
			c.AbortWithStatusJSON(401, gin.H{"error": "sign in required"})
			return false
		}
		c.AbortWithStatusJSON(503, gin.H{"error": "authentication temporarily unavailable"})
		return false
	}
	if c.Request.Method != "GET" && c.Request.Method != "HEAD" {
		if subtle.ConstantTimeCompare([]byte(c.GetHeader("X-CSRF-Token")), []byte(models.PortalCSRF(token))) != 1 {
			c.AbortWithStatusJSON(403, gin.H{"error": "invalid request token"})
			return false
		}
	}
	if u.MustChangePassword && !allowPasswordChange {
		c.AbortWithStatusJSON(403, gin.H{"error": "change your temporary password before continuing", "code": "password_change_required"})
		return false
	}
	c.Set(portalUserContext, u)
	c.Set(portalExpiryContext, expires)
	// Renew the persistent cookie only with a renewed database expiry. Avoid
	// needless Set-Cookie headers on concurrent polling responses. Logout and
	// password change set their own final cookie after committing.
	if renewed && c.Request.URL.Path != "/auth/logout" && c.Request.URL.Path != "/auth/password" {
		setPortalCookie(c, token, expires, time.Now().UTC())
	}
	return true
}
func portalJSON(c *gin.Context, out any) bool {
	if !strings.HasPrefix(strings.ToLower(c.ContentType()), "application/json") {
		c.JSON(400, gin.H{"error": "JSON body required"})
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 8<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(out) != nil || dec.Decode(new(any)) != io.EOF {
		c.JSON(400, gin.H{"error": "invalid request"})
		return false
	}
	return true
}
func portalError(c *gin.Context, e error) {
	var mysqlError *driver.MySQLError
	switch {
	case errors.Is(e, models.ErrPortalInput):
		c.JSON(400, gin.H{"error": "invalid account input; usernames 3–32 lowercase ASCII characters, passwords 15–128 characters, roles admin/viewer"})
	case errors.Is(e, models.ErrPortalForbidden):
		c.JSON(403, gin.H{"error": "not authorized; confirm your current password"})
	case errors.Is(e, models.ErrPortalLastAdmin):
		c.JSON(409, gin.H{"error": "at least one active administrator must remain"})
	case errors.Is(e, models.ErrPortalConflict):
		c.JSON(409, gin.H{"error": "account changed; refresh and retry deliberately"})
	case errors.As(e, &mysqlError) && mysqlError.Number == 1062:
		c.JSON(409, gin.H{"error": "username is already in use"})
	case errors.Is(e, gorm.ErrRecordNotFound):
		c.JSON(404, gin.H{"error": "account not found"})
	default:
		c.JSON(503, gin.H{"error": "account operation unavailable; inspect current state before retrying"})
	}
}

type portalLimitEntry struct {
	Until time.Time
	Count int
}

var portalLimits = struct {
	sync.Mutex
	Items map[string]portalLimitEntry
}{Items: map[string]portalLimitEntry{}}
var portalHashSlots = make(chan struct{}, 2)

func portalClientIP(c *gin.Context) string {
	host, _, e := net.SplitHostPort(c.Request.RemoteAddr)
	if e != nil {
		return "unknown"
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() && trustedPortalGateway(c) {
		if real := net.ParseIP(c.GetHeader("X-Real-IP")); real != nil {
			return real.String()
		}
	}
	return host
}
func allowPortalAttempt(c *gin.Context, username string) bool {
	now := time.Now()
	ip := portalClientIP(c)
	keys := []string{models.PortalTokenHash("ip:" + ip), models.PortalTokenHash("name-ip:" + username + ":" + ip)}
	limits := []int{60, 10}
	portalLimits.Lock()
	defer portalLimits.Unlock()
	for k, v := range portalLimits.Items {
		if !now.Before(v.Until) {
			delete(portalLimits.Items, k)
		}
	}
	if len(portalLimits.Items) > 4092 {
		return false
	}
	for i, k := range keys {
		v := portalLimits.Items[k]
		if now.Before(v.Until) && v.Count >= limits[i] {
			return false
		}
	}
	for _, k := range keys {
		v := portalLimits.Items[k]
		if !now.Before(v.Until) {
			v = portalLimitEntry{Until: now.Add(15 * time.Minute)}
		}
		v.Count++
		portalLimits.Items[k] = v
	}
	return true
}
func portalCostSlot(c *gin.Context) bool {
	select {
	case portalHashSlots <- struct{}{}:
		return true
	default:
		c.Header("Retry-After", "2")
		c.JSON(503, gin.H{"error": "sign-in busy; try again shortly"})
		return false
	}
}
func RegisterPortalAuthRoutes(r *gin.Engine) {
	r.GET("/auth/session", portalSession)
	r.POST("/auth/login", portalLogin)
	r.POST("/auth/logout", portalLogout)
	r.POST("/auth/password", portalPassword)
	g := r.Group("/auth/users", func(c *gin.Context) {
		if !portalSessionAuth(c, false) {
			return
		}
		u, _ := portalAccount(c)
		if u.Role != "admin" {
			c.AbortWithStatusJSON(403, gin.H{"error": "administrator required"})
			return
		}
		c.Next()
	})
	g.GET("", portalUsers)
	g.POST("", portalCreateUser)
	g.PATCH("/:id", portalUpdateUser)
	g.POST("/:id/reset-password", portalResetPassword)
	g.POST("/:id/revoke", portalRevoke)
}
func portalSession(c *gin.Context) {
	portalNoStore(c)
	if PortalAuthMode() == "legacy" {
		if !validAdminToken(c) || !portalOriginOK(c, false) {
			c.JSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.JSON(200, gin.H{"mode": "legacy", "user": gin.H{"username": "Legacy administrator", "role": "admin", "must_change_password": false}, "csrf": ""})
		return
	}
	if !portalSessionAuth(c, true) {
		return
	}
	u, _ := portalAccount(c)
	expires, _ := c.Get(portalExpiryContext)
	absolute, renewal := 0, int(models.PortalSessionRenewalInterval.Seconds())
	if u.MustChangePassword {
		absolute, renewal = int(models.PortalTemporarySessionLifetime.Seconds()), 0
	}
	c.JSON(200, gin.H{"mode": "session", "user": u, "csrf": models.PortalCSRF(portalRawToken(c)), "idle_seconds": 0, "absolute_seconds": absolute, "persistent_seconds": int(models.PortalSessionDuration(u).Seconds()), "renewal_seconds": renewal, "expires_at": expires})
}
func portalLogin(c *gin.Context) {
	if !portalGateway(c) {
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !portalJSON(c, &in) {
		return
	}
	name, e := models.PortalUsername(in.Username)
	validName := e == nil
	if e != nil {
		name = "invalid-username"
	}
	if len(in.Password) == 0 || len(in.Password) > 512 {
		c.JSON(401, gin.H{"error": "invalid username or password"})
		return
	}
	if !allowPortalAttempt(c, name) {
		c.Header("Retry-After", "900")
		c.JSON(429, gin.H{"error": "too many attempts; try again later"})
		return
	}
	if !portalCostSlot(c) {
		return
	}
	defer func() { <-portalHashSlots }()
	if !validName {
		models.DummyPortalPasswordCheck(in.Password)
		c.JSON(401, gin.H{"error": "invalid username or password"})
		return
	}
	u, err := models.PortalFindUser(mcpDB(c), name)
	valid := false
	if err == nil {
		valid = models.VerifyPortalPassword(u.PasswordHash, in.Password)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		models.DummyPortalPasswordCheck(in.Password)
	} else {
		c.JSON(503, gin.H{"error": "sign-in temporarily unavailable"})
		return
	}
	if !valid || u.Disabled {
		c.JSON(401, gin.H{"error": "invalid username or password"})
		return
	}
	now := time.Now().UTC()
	u, token, err := models.CreatePortalSession(mcpDB(c), u.ID, u.Version, now, portalRawToken(c))
	if err != nil {
		c.JSON(503, gin.H{"error": "sign-in temporarily unavailable"})
		return
	}
	// Existing client-supplied cookie is never adopted; every login gets fresh entropy.
	setPortalCookie(c, token, now.Add(models.PortalSessionDuration(u)), time.Now().UTC())
	c.JSON(200, gin.H{"mode": "session", "user": u, "csrf": models.PortalCSRF(token)})
}
func portalLogout(c *gin.Context) {
	if !portalSessionAuth(c, true) {
		return
	}
	u, _ := portalAccount(c)
	if e := models.LogoutPortalSession(mcpDB(c), portalRawToken(c), u.ID, time.Now().UTC()); e != nil {
		portalError(c, e)
		return
	}
	clearPortalCookie(c)
	c.JSON(200, gin.H{"logged_out": true})
}
func portalPassword(c *gin.Context) {
	if !portalSessionAuth(c, true) {
		return
	}
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !portalJSON(c, &in) {
		return
	}
	u, _ := portalAccount(c)
	if !allowPortalAttempt(c, u.Username) {
		c.JSON(429, gin.H{"error": "too many attempts"})
		return
	}
	if !portalCostSlot(c) {
		return
	}
	defer func() { <-portalHashSlots }()
	updated, e := models.ChangePortalPassword(mcpDB(c), u.ID, u.Version, in.CurrentPassword, in.NewPassword, time.Now().UTC())
	if e != nil {
		portalError(c, e)
		return
	}
	now := time.Now().UTC()
	updated, token, e := models.CreatePortalSession(mcpDB(c), updated.ID, updated.Version, now)
	if e != nil {
		clearPortalCookie(c)
		c.JSON(503, gin.H{"error": "password changed; sign in again", "code": "password_changed_sign_in"})
		return
	}
	setPortalCookie(c, token, now.Add(models.PortalSessionDuration(updated)), time.Now().UTC())
	c.JSON(200, gin.H{"user": updated, "csrf": models.PortalCSRF(token), "mode": "session"})
}
func portalUsers(c *gin.Context) {
	items, e := models.ListPortalUsers(mcpDB(c))
	if e != nil {
		portalError(c, e)
		return
	}
	c.JSON(200, gin.H{"items": items})
}
func portalSensitive(c *gin.Context) (models.PortalUser, bool) {
	u, _ := portalAccount(c)
	if !allowPortalAttempt(c, u.Username) {
		c.JSON(429, gin.H{"error": "too many attempts"})
		return u, false
	}
	return u, portalCostSlot(c)
}
func portalCreateUser(c *gin.Context) {
	var in struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if !portalJSON(c, &in) {
		return
	}
	u, ok := portalSensitive(c)
	if !ok {
		return
	}
	defer func() { <-portalHashSlots }()
	password, e := models.PortalRandomSecret()
	if e != nil {
		portalError(c, e)
		return
	}
	created, e := models.CreatePortalUser(mcpDB(c), u.ID, u.Version, in.Username, in.Role, password, time.Now().UTC())
	if e != nil {
		portalError(c, e)
		return
	}
	c.JSON(200, gin.H{"user": created, "temporary_password": password, "display_once": true})
}
func portalUpdateUser(c *gin.Context) {
	id, ok := mcpID(c)
	if !ok {
		return
	}
	var in struct {
		Role     *string `json:"role"`
		Disabled *bool   `json:"disabled"`
		Version  uint    `json:"version"`
	}
	if !portalJSON(c, &in) {
		return
	}
	u, ok := portalSensitive(c)
	if !ok {
		return
	}
	defer func() { <-portalHashSlots }()
	updated, e := models.UpdatePortalUser(mcpDB(c), u.ID, u.Version, uint(id), in.Version, in.Role, in.Disabled, time.Now().UTC())
	if e != nil {
		portalError(c, e)
		return
	}
	c.JSON(200, gin.H{"user": updated, "sessions_revoked": updated.Version != in.Version})
}
func portalResetPassword(c *gin.Context) {
	id, ok := mcpID(c)
	if !ok {
		return
	}
	var in struct {
		Version uint `json:"version"`
	}
	if !portalJSON(c, &in) {
		return
	}
	u, ok := portalSensitive(c)
	if !ok {
		return
	}
	defer func() { <-portalHashSlots }()
	password, e := models.PortalRandomSecret()
	if e != nil {
		portalError(c, e)
		return
	}
	updated, e := models.ResetPortalUserPassword(mcpDB(c), u.ID, u.Version, uint(id), in.Version, password, time.Now().UTC())
	if e != nil {
		portalError(c, e)
		return
	}
	c.JSON(200, gin.H{"user": updated, "temporary_password": password, "display_once": true, "sessions_revoked": true})
}
func portalRevoke(c *gin.Context) {
	id, ok := mcpID(c)
	if !ok {
		return
	}
	var in struct {
		Version uint `json:"version"`
	}
	if !portalJSON(c, &in) {
		return
	}
	u, ok := portalSensitive(c)
	if !ok {
		return
	}
	defer func() { <-portalHashSlots }()
	if e := models.RevokePortalUserSessions(mcpDB(c), u.ID, u.Version, uint(id), in.Version, time.Now().UTC()); e != nil {
		portalError(c, e)
		return
	}
	c.JSON(200, gin.H{"sessions_revoked": true})
}
