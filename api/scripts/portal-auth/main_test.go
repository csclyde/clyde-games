package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCredentialFileIsPrivateExclusiveAndNeverSourceOutput(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	password := "fixture credential value only"
	if e := writeCredential(dir, "initial.txt", "admin", password); e != nil {
		t.Fatal("private file creation failed")
	}
	raw, e := os.ReadFile(filepath.Join(dir, "initial.txt"))
	if e != nil || !strings.Contains(string(raw), password) {
		t.Fatal("private output missing")
	}
	info, _ := os.Stat(filepath.Join(dir, "initial.txt"))
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatal("credential permissions")
	}
	if writeCredential(dir, "initial.txt", "admin", "other fixture password") == nil {
		t.Fatal("credential overwrite allowed")
	}
	for _, name := range []string{"../outside.txt", "sub/file.txt", "", "a.log"} {
		if writeCredential(dir, name, "admin", password) == nil {
			t.Fatal("unsafe credential filename accepted")
		}
	}
}
func TestRehearsalRejectsLiveAndArbitraryDatabase(t *testing.T) {
	for _, name := range []string{"analytics", "", "copy", "portal_auth_rehearsal"} {
		if rehearse(nil, name) == nil {
			t.Fatal("unsafe rehearsal target allowed")
		}
	}
}
func TestCredentialDirectoryAndSymlinkGuards(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux bootstrap filesystem guarantees")
	}
	root := t.TempDir()
	unsafe := filepath.Join(root, "unsafe")
	if os.Mkdir(unsafe, 0o755) != nil || os.Chmod(unsafe, 0o755) != nil {
		t.Fatal("fixture setup failed")
	}
	if writeCredential(unsafe, "initial.txt", "admin", "fixture passphrase only") == nil {
		t.Fatal("insecure directory accepted")
	}
	private := filepath.Join(root, "private")
	if os.Mkdir(private, 0o700) != nil {
		t.Fatal("fixture setup failed")
	}
	link := filepath.Join(root, "link")
	if os.Symlink(private, link) != nil {
		t.Fatal("fixture symlink failed")
	}
	if writeCredential(link, "initial.txt", "admin", "fixture passphrase only") == nil {
		t.Fatal("symlink directory accepted")
	}
	target := filepath.Join(root, "untouched")
	if os.WriteFile(target, []byte("original fixture"), 0o600) != nil {
		t.Fatal("fixture setup failed")
	}
	if os.Symlink(target, filepath.Join(private, "initial.txt")) != nil {
		t.Fatal("fixture symlink failed")
	}
	if writeCredential(private, "initial.txt", "admin", "fixture passphrase only") == nil {
		t.Fatal("credential symlink overwrite allowed")
	}
	data, e := os.ReadFile(target)
	if e != nil || string(data) != "original fixture" {
		t.Fatal("existing evidence overwritten")
	}
}
