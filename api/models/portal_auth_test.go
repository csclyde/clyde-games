package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPortalPasswordHashAndBoundedVerification(t *testing.T) {
	password := "fixture passphrase for tests only"
	a, e := HashPortalPassword(password)
	if e != nil {
		t.Fatal("hash failed")
	}
	b, e := HashPortalPassword(password)
	if e != nil || a == b {
		t.Fatal("salt must differ")
	}
	if !strings.HasPrefix(a, "$argon2id$v=19$m=19456,t=2,p=1$") || !VerifyPortalPassword(a, password) || VerifyPortalPassword(a, password+"wrong") {
		t.Fatal("password verification failed")
	}
	for _, bad := range []string{"", "$argon2id$v=19$m=4294967295,t=2,p=1$AA$AA", strings.Replace(a, "t=2", "t=999", 1), strings.Replace(a, "p=1", "p=255", 1), strings.Replace(a, "v=19", "v=1", 1)} {
		if VerifyPortalPassword(bad, password) {
			t.Fatal("invalid hash accepted")
		}
	}
	for _, bad := range []string{"short", strings.Repeat("a", 129), string([]byte{0xff})} {
		if _, e := HashPortalPassword(bad); e == nil {
			t.Fatal("invalid new password accepted")
		}
	}
	for _, bad := range []string{"a", "1admin", "a user", "../admin", strings.Repeat("x", 33)} {
		if _, e := PortalUsername(bad); e == nil {
			t.Fatal("invalid username accepted")
		}
	}
	if name, e := PortalUsername(" Admin "); e != nil || name != "admin" {
		t.Fatal("username normalization")
	}
	token, e := PortalRandomSecret()
	if e != nil || len(token) != 43 || len(PortalTokenHash(token)) != 64 || PortalCSRF(token) == token || len(PortalCSRF(token)) != 43 {
		t.Fatal("session entropy/derivation")
	}
	raw, _ := json.Marshal(PortalUser{ID: 1, Username: "admin", Role: "admin", PasswordHash: a})
	if strings.Contains(string(raw), a) || strings.Contains(string(raw), password) || strings.Contains(string(raw), "password_hash") {
		t.Fatal("credential material serialized")
	}
}
