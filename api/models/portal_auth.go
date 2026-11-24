package models

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// Normal browser sessions survive restarts and renew during use. Temporary
// password sessions retain a short, non-renewing lifetime and no private access.
const PortalSessionLifetime = 365 * 24 * time.Hour
const PortalSessionRenewalInterval = 24 * time.Hour
const PortalTemporarySessionLifetime = 8 * time.Hour

func PortalSessionDuration(user PortalUser) time.Duration {
	if user.MustChangePassword {
		return PortalTemporarySessionLifetime
	}
	return PortalSessionLifetime
}

const portalMemory = 19 * 1024

var ErrPortalInput = errors.New("invalid account input")
var ErrPortalAuth = errors.New("authentication required")
var ErrPortalForbidden = errors.New("not authorized")
var ErrPortalConflict = errors.New("account changed; refresh before retrying")
var ErrPortalLastAdmin = errors.New("at least one active administrator must remain")

type PortalUser struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	Username           string    `gorm:"type:varbinary(32);uniqueIndex;not null" json:"username"`
	Role               string    `gorm:"type:varchar(16);not null" json:"role"`
	Disabled           bool      `json:"disabled"`
	MustChangePassword bool      `json:"must_change_password"`
	Version            uint      `gorm:"not null;default:1" json:"version"`
	PasswordHash       string    `gorm:"type:varchar(255);not null" json:"-"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
type PortalSession struct {
	TokenHash   string     `gorm:"type:char(64);primaryKey" json:"-"`
	UserID      uint       `gorm:"index;not null" json:"-"`
	UserVersion uint       `json:"-"`
	CreatedAt   time.Time  `json:"-"`
	LastSeenAt  time.Time  `gorm:"index" json:"-"`
	ExpiresAt   time.Time  `gorm:"index" json:"-"`
	User        PortalUser `gorm:"constraint:OnDelete:RESTRICT" json:"-"`
}
type PortalAuthControl struct {
	ID uint `gorm:"primaryKey"`
}
type PortalAuthEvent struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	ActorID   uint
	SubjectID uint
	Action    string `gorm:"type:varchar(32)"`
}

func portalDB(db *gorm.DB) *gorm.DB {
	return db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}
func MigratePortalAuth(db *gorm.DB) error {
	db = portalDB(db)
	if e := db.AutoMigrate(&PortalUser{}, &PortalSession{}, &PortalAuthControl{}, &PortalAuthEvent{}); e != nil {
		return e
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&PortalAuthControl{ID: 1}).Error
}
func PortalUsername(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) < 3 || len(s) > 32 || s[0] < 'a' || s[0] > 'z' {
		return "", ErrPortalInput
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune("_.-", r)) {
			return "", ErrPortalInput
		}
	}
	return s, nil
}
func ValidPortalPassword(s string) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) >= 15 && utf8.RuneCountInString(s) <= 128 && len(s) <= 512
}
func PortalRandomSecret() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func HashPortalPassword(password string) (string, error) {
	if !ValidPortalPassword(password) {
		return "", ErrPortalInput
	}
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	key := argon2.IDKey([]byte(password), salt, 2, portalMemory, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=2,p=1$%s$%s", portalMemory, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}
func VerifyPortalPassword(encoded, password string) bool {
	if len(password) > 512 || !utf8.ValidString(password) {
		return false
	}
	p := strings.Split(encoded, "$")
	if len(p) != 6 || p[0] != "" || p[1] != "argon2id" || p[2] != "v=19" {
		return false
	}
	params := strings.Split(p[3], ",")
	if len(params) != 3 {
		return false
	}
	values := make([]uint64, 3)
	for i, prefix := range []string{"m=", "t=", "p="} {
		if !strings.HasPrefix(params[i], prefix) {
			return false
		}
		v, e := strconv.ParseUint(strings.TrimPrefix(params[i], prefix), 10, 32)
		if e != nil {
			return false
		}
		values[i] = v
	}
	if values[0] < portalMemory || values[0] > 128*1024 || values[1] < 1 || values[1] > 6 || values[2] < 1 || values[2] > 4 {
		return false
	}
	salt, e := base64.RawStdEncoding.Strict().DecodeString(p[4])
	if e != nil || len(salt) < 16 || len(salt) > 32 {
		return false
	}
	want, e := base64.RawStdEncoding.Strict().DecodeString(p[5])
	if e != nil || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, uint32(values[1]), uint32(values[0]), uint8(values[2]), 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}
func DummyPortalPasswordCheck(password string) {
	argon2.IDKey([]byte(password), []byte("portal-dummy-salt"), 2, portalMemory, 1, 32)
}
func PortalTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func PortalCSRF(token string) string {
	h := hmac.New(sha256.New, []byte(token))
	h.Write([]byte("clyde-portal-csrf-v1"))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
func PortalFindUser(db *gorm.DB, username string) (PortalUser, error) {
	var u PortalUser
	e := portalDB(db).Where("username = ?", username).Take(&u).Error
	return u, e
}
func auditPortal(tx *gorm.DB, actor, subject uint, action string, now time.Time) error {
	return tx.Create(&PortalAuthEvent{CreatedAt: now.UTC(), ActorID: actor, SubjectID: subject, Action: action}).Error
}
func CreatePortalSession(db *gorm.DB, id, version uint, now time.Time, priorTokens ...string) (PortalUser, string, error) {
	var u PortalUser
	token, e := PortalRandomSecret()
	if e != nil {
		return u, "", e
	}
	now = now.UTC()
	e = portalDB(db).Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, id).Error; e != nil {
			return e
		}
		if u.Disabled || u.Version != version || (u.Role != "admin" && u.Role != "viewer") {
			return ErrPortalAuth
		}
		if e := tx.Create(&PortalSession{TokenHash: PortalTokenHash(token), UserID: id, UserVersion: version, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(PortalSessionDuration(u))}).Error; e != nil {
			return e
		}
		for _, prior := range priorTokens {
			if len(prior) == 43 {
				if e := tx.Where("token_hash = ? AND user_id = ?", PortalTokenHash(prior), id).Delete(&PortalSession{}).Error; e != nil {
					return e
				}
			}
		}
		return auditPortal(tx, id, id, "login", now)
	})
	if e != nil {
		return PortalUser{}, "", e
	}
	return u, token, nil
}

// Keep the existing caller contract; HTTP callers also need the authoritative
// expiry so browser persistence and server-side validity remain aligned.
func AuthenticatePortalSession(db *gorm.DB, token string, now time.Time) (PortalUser, error) {
	u, _, _, err := AuthenticatePortalSessionWithExpiry(db, token, now)
	return u, err
}

func AuthenticatePortalSessionWithExpiry(db *gorm.DB, token string, now time.Time) (PortalUser, time.Time, bool, error) {
	if len(token) != 43 {
		return PortalUser{}, time.Time{}, false, ErrPortalAuth
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 32 {
		return PortalUser{}, time.Time{}, false, ErrPortalAuth
	}
	now = now.UTC()
	db = portalDB(db)
	hash := PortalTokenHash(token)
	read := func() (PortalUser, PortalSession, error) {
		var session PortalSession
		if err := db.Where("token_hash = ?", hash).Take(&session).Error; err != nil {
			return PortalUser{}, session, err
		}
		if !now.Before(session.ExpiresAt) || now.Before(session.CreatedAt) {
			return PortalUser{}, session, ErrPortalAuth
		}
		var user PortalUser
		if err := db.First(&user, session.UserID).Error; err != nil {
			return PortalUser{}, session, err
		}
		if user.Disabled || user.Version != session.UserVersion || (user.Role != "admin" && user.Role != "viewer") {
			return PortalUser{}, session, ErrPortalAuth
		}
		// Only still-valid sessions may upgrade from the original policy. A
		// legacy eight-hour normal session that already hit its old idle timeout
		// cannot gain a new year merely because the policy was deployed later.
		legacy := !user.MustChangePassword && !session.ExpiresAt.After(session.CreatedAt.Add(PortalTemporarySessionLifetime+time.Second))
		if legacy && !session.LastSeenAt.After(now.Add(-30*time.Minute)) {
			return PortalUser{}, session, ErrPortalAuth
		}
		return user, session, nil
	}
	u, session, err := read()
	if err != nil {
		return PortalUser{}, time.Time{}, false, err
	}
	renew := !u.MustChangePassword && !session.ExpiresAt.After(now.Add(PortalSessionLifetime-PortalSessionRenewalInterval))
	if renew || now.Sub(session.LastSeenAt) >= time.Minute {
		// Concurrent requests can legitimately update the same timestamp, producing
		// zero changed rows in MySQL. Never treat that as proof of revocation.
		// Monotonic UPDATEs cannot recreate a deleted session or shorten its expiry.
		updates := map[string]interface{}{"last_seen_at": gorm.Expr("GREATEST(last_seen_at, ?)", now)}
		if renew {
			updates["expires_at"] = gorm.Expr("GREATEST(expires_at, ?)", now.Add(PortalSessionLifetime))
		}
		result := db.Model(&PortalSession{}).Where("token_hash = ? AND user_version = ? AND expires_at > ? AND created_at <= ?", hash, session.UserVersion, now, now).Updates(updates)
		if result.Error != nil {
			return PortalUser{}, time.Time{}, false, result.Error
		}
		// Re-read for both changed and unchanged updates. A real logout, role or
		// password change, account disable, or revocation still fails closed.
		u, session, err = read()
		if err != nil {
			return PortalUser{}, time.Time{}, false, err
		}
	}
	return u, session.ExpiresAt, renew, nil
}
func LogoutPortalSession(db *gorm.DB, token string, id uint, now time.Time) error {
	return portalDB(db).Transaction(func(tx *gorm.DB) error {
		r := tx.Where("token_hash = ? AND user_id = ?", PortalTokenHash(token), id).Delete(&PortalSession{})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			return nil
		}
		return auditPortal(tx, id, id, "logout", now)
	})
}
func ListPortalUsers(db *gorm.DB) ([]PortalUser, error) {
	out := []PortalUser{}
	e := portalDB(db).Omit("password_hash").Order("id ASC").Limit(201).Find(&out).Error
	if len(out) > 200 {
		return nil, errors.New("user list exceeds supported bound")
	}
	return out, e
}
func lockPortalControl(tx *gorm.DB) error {
	var c PortalAuthControl
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&c, 1).Error
}
func verifyPortalActor(tx *gorm.DB, id, version uint, password string, admin bool) (PortalUser, error) {
	var u PortalUser
	e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, id).Error
	if e != nil {
		return u, e
	}
	if u.Disabled || u.Version != version || (admin && (u.Role != "admin" || u.MustChangePassword)) || (!admin && !VerifyPortalPassword(u.PasswordHash, password)) {
		return PortalUser{}, ErrPortalForbidden
	}
	return u, nil
}
func CreatePortalUser(db *gorm.DB, actorID, actorVersion uint, username, role, password string, now time.Time) (PortalUser, error) {
	var out PortalUser
	name, e := PortalUsername(username)
	if e != nil || (role != "admin" && role != "viewer") {
		return out, ErrPortalInput
	}
	hash, e := HashPortalPassword(password)
	if e != nil {
		return out, e
	}
	e = portalDB(db).Transaction(func(tx *gorm.DB) error {
		if e := lockPortalControl(tx); e != nil {
			return e
		}
		if _, e := verifyPortalActor(tx, actorID, actorVersion, "", true); e != nil {
			return e
		}
		var count int64
		if e := tx.Model(&PortalUser{}).Count(&count).Error; e != nil {
			return e
		}
		if count >= 200 {
			return ErrPortalInput
		}
		out = PortalUser{Username: name, Role: role, PasswordHash: hash, Version: 1, MustChangePassword: true}
		if e := tx.Create(&out).Error; e != nil {
			return e
		}
		return auditPortal(tx, actorID, out.ID, "user_created", now)
	})
	return out, e
}
func UpdatePortalUser(db *gorm.DB, actorID, actorVersion, targetID, expectedVersion uint, role *string, disabled *bool, now time.Time) (PortalUser, error) {
	var out PortalUser
	if expectedVersion == 0 || (role == nil && disabled == nil) || (role != nil && *role != "admin" && *role != "viewer") {
		return out, ErrPortalInput
	}
	e := portalDB(db).Transaction(func(tx *gorm.DB) error {
		if e := lockPortalControl(tx); e != nil {
			return e
		}
		if _, e := verifyPortalActor(tx, actorID, actorVersion, "", true); e != nil {
			return e
		}
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&out, targetID).Error; e != nil {
			return e
		}
		if out.Version != expectedVersion {
			return ErrPortalConflict
		}
		nextRole, nextDisabled := out.Role, out.Disabled
		if role != nil {
			nextRole = *role
		}
		if disabled != nil {
			nextDisabled = *disabled
		}
		if out.Role == nextRole && out.Disabled == nextDisabled {
			return nil
		}
		if out.Role == "admin" && !out.Disabled && (nextRole != "admin" || nextDisabled) {
			var count int64
			if e := tx.Model(&PortalUser{}).Where("role = ? AND disabled = ?", "admin", false).Count(&count).Error; e != nil {
				return e
			}
			if count <= 1 {
				return ErrPortalLastAdmin
			}
		}
		out.Role = nextRole
		out.Disabled = nextDisabled
		out.Version++
		if e := tx.Model(&PortalUser{}).Where("id = ?", out.ID).Updates(map[string]any{"role": out.Role, "disabled": out.Disabled, "version": out.Version}).Error; e != nil {
			return e
		}
		if e := tx.Where("user_id = ?", out.ID).Delete(&PortalSession{}).Error; e != nil {
			return e
		}
		return auditPortal(tx, actorID, out.ID, "user_updated", now)
	})
	return out, e
}
func ResetPortalUserPassword(db *gorm.DB, actorID, actorVersion, targetID, expectedVersion uint, newPassword string, now time.Time) (PortalUser, error) {
	var out PortalUser
	if actorID == targetID || expectedVersion == 0 {
		return out, ErrPortalInput
	}
	hash, e := HashPortalPassword(newPassword)
	if e != nil {
		return out, e
	}
	e = portalDB(db).Transaction(func(tx *gorm.DB) error {
		if e := lockPortalControl(tx); e != nil {
			return e
		}
		if _, e := verifyPortalActor(tx, actorID, actorVersion, "", true); e != nil {
			return e
		}
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&out, targetID).Error; e != nil {
			return e
		}
		if out.Version != expectedVersion {
			return ErrPortalConflict
		}
		out.Version++
		out.MustChangePassword = true
		if e := tx.Model(&PortalUser{}).Where("id = ?", out.ID).Updates(map[string]any{"password_hash": hash, "must_change_password": true, "version": out.Version}).Error; e != nil {
			return e
		}
		if e := tx.Where("user_id = ?", out.ID).Delete(&PortalSession{}).Error; e != nil {
			return e
		}
		return auditPortal(tx, actorID, out.ID, "password_reset", now)
	})
	return out, e
}
func RevokePortalUserSessions(db *gorm.DB, actorID, actorVersion, targetID, expectedVersion uint, now time.Time) error {
	return portalDB(db).Transaction(func(tx *gorm.DB) error {
		if e := lockPortalControl(tx); e != nil {
			return e
		}
		if _, e := verifyPortalActor(tx, actorID, actorVersion, "", true); e != nil {
			return e
		}
		var u PortalUser
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, targetID).Error; e != nil {
			return e
		}
		if u.Version != expectedVersion {
			return ErrPortalConflict
		}
		if e := tx.Model(&u).UpdateColumn("version", u.Version+1).Error; e != nil {
			return e
		}
		if e := tx.Where("user_id = ?", u.ID).Delete(&PortalSession{}).Error; e != nil {
			return e
		}
		return auditPortal(tx, actorID, u.ID, "sessions_revoked", now)
	})
}
func ChangePortalPassword(db *gorm.DB, id, version uint, currentPassword, newPassword string, now time.Time) (PortalUser, error) {
	var out PortalUser
	if currentPassword == newPassword {
		return out, ErrPortalInput
	}
	hash, e := HashPortalPassword(newPassword)
	if e != nil {
		return out, e
	}
	e = portalDB(db).Transaction(func(tx *gorm.DB) error {
		if e := lockPortalControl(tx); e != nil {
			return e
		}
		u, e := verifyPortalActor(tx, id, version, currentPassword, false)
		if e != nil {
			return e
		}
		out = u
		out.Version++
		out.MustChangePassword = false
		if e = tx.Model(&out).Updates(map[string]any{"password_hash": hash, "must_change_password": false, "version": out.Version}).Error; e != nil {
			return e
		}
		if e = tx.Where("user_id = ?", id).Delete(&PortalSession{}).Error; e != nil {
			return e
		}
		return auditPortal(tx, id, id, "password_changed", now)
	})
	return out, e
}

// Bootstrap/recovery is CLI-only; never called by migrations or an HTTP route.
func BootstrapPortalAdmin(db *gorm.DB, password string, now time.Time) (PortalUser, error) {
	var u PortalUser
	hash, e := HashPortalPassword(password)
	if e != nil {
		return u, e
	}
	e = portalDB(db).Transaction(func(tx *gorm.DB) error {
		if e := lockPortalControl(tx); e != nil {
			return e
		}
		var count int64
		if e := tx.Model(&PortalUser{}).Count(&count).Error; e != nil {
			return e
		}
		if count != 0 {
			return ErrPortalConflict
		}
		u = PortalUser{Username: "admin", Role: "admin", PasswordHash: hash, Version: 1, MustChangePassword: true}
		if e := tx.Create(&u).Error; e != nil {
			return e
		}
		return auditPortal(tx, 0, u.ID, "bootstrap", now)
	})
	return u, e
}
func RecoverPortalAdmin(db *gorm.DB, username, password string, now time.Time) error {
	name, e := PortalUsername(username)
	if e != nil {
		return e
	}
	hash, e := HashPortalPassword(password)
	if e != nil {
		return e
	}
	return portalDB(db).Transaction(func(tx *gorm.DB) error {
		if e := lockPortalControl(tx); e != nil {
			return e
		}
		var u PortalUser
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("username = ?", name).Take(&u).Error; e != nil {
			return e
		}
		if u.Role != "admin" {
			return ErrPortalForbidden
		}
		if e := tx.Model(&u).Updates(map[string]any{"password_hash": hash, "must_change_password": true, "disabled": false, "version": u.Version + 1}).Error; e != nil {
			return e
		}
		if e := tx.Where("user_id = ?", u.ID).Delete(&PortalSession{}).Error; e != nil {
			return e
		}
		return auditPortal(tx, 0, u.ID, "admin_recovery", now)
	})
}
