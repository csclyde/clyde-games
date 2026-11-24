// Operator-only credential bootstrap/recovery. No HTTP signup/bootstrap route.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"api.clyde.games/models"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const secretDir = "/root/clyde-portal-secrets"

var secretName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,79}\.txt$`)

func writeCredential(dir, name, username, password string) error {
	if !secretName.MatchString(name) || filepath.Base(name) != name {
		return errors.New("invalid private credential filename")
	}
	if e := os.MkdirAll(dir, 0o700); e != nil {
		return errors.New("cannot create private credential directory")
	}
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || (runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0) {
		return errors.New("credential directory must be a private non-symlink directory")
	}
	f, e := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if e != nil {
		return errors.New("cannot exclusively create credential file; inspect existing state before retrying")
	}
	_, e = fmt.Fprintf(f, "Site: https://admin.clyde.games\nUsername: %s\nTemporary password: %s\nChange this password at first login. Transfer privately into the owner's password manager; never paste into chat or source.\n", username, password)
	if e == nil {
		e = f.Sync()
	}
	closeError := f.Close()
	if e != nil || closeError != nil {
		return errors.New("credential file write failed; inspect private file without printing it")
	}
	if runtime.GOOS == "linux" {
		// Persist the file entry and, if newly created, the private directory's
		// entry before a database commit can make these credentials necessary.
		for _, path := range []string{dir, filepath.Dir(dir)} {
			directory, err := os.Open(path)
			if err != nil {
				return errors.New("credential directory durability not confirmed; inspect retained private file before retry")
			}
			err = directory.Sync()
			closeErr := directory.Close()
			if err != nil || closeErr != nil {
				return errors.New("credential directory durability not confirmed; inspect retained private file before retry")
			}
		}
	}
	return nil
}
func run() error {
	action := flag.String("action", "", "migrate, rehearse, bootstrap, or recover-admin")
	filename := flag.String("credential-name", "", "new filename under the fixed private credential directory")
	username := flag.String("username", "admin", "existing administrator for recovery")
	live := flag.Bool("allow-live", false, "explicit operator acknowledgement for analytics database; requires separate owner approval")
	flag.Parse()
	if *action != "migrate" && *action != "rehearse" && *action != "bootstrap" && *action != "recover-admin" {
		return errors.New("explicit action required")
	}
	cfg, e := driver.ParseDSN(os.Getenv("PORTAL_AUTH_DSN"))
	if e != nil || cfg == nil || cfg.DBName == "" || cfg.Net != "tcp" || (cfg.Addr != "127.0.0.1:3306" && cfg.Addr != "[::1]:3306") {
		return errors.New("private explicit loopback database DSN required")
	}
	if cfg.DBName == "analytics" && !*live {
		return errors.New("live database requires action-time owner approval and allow-live acknowledgement")
	}
	if runtime.GOOS != "linux" && *action != "migrate" && *action != "rehearse" {
		return errors.New("credential bootstrap/recovery is supported only on the Linux server")
	}
	db, e := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		return errors.New("authentication database unavailable")
	}
	if *action == "rehearse" {
		return rehearse(db, cfg.DBName)
	}
	if *action == "migrate" {
		if models.MigratePortalAuth(db) != nil {
			return errors.New("authentication schema migration failed")
		}
		fmt.Println("Authentication schema ready; no user credentials created.")
		return nil
	}
	if !secretName.MatchString(*filename) {
		return errors.New("private credential filename is required")
	}
	name := "admin"
	if *action == "recover-admin" {
		name, e = models.PortalUsername(*username)
		if e != nil {
			return e
		}
	}
	password, e := models.PortalRandomSecret()
	if e != nil {
		return errors.New("secure randomness unavailable")
	}
	// File first: a commit/network error must never destroy the sole usable secret.
	// Existing files are never overwritten; an uncertain operation is inspected,
	// not blindly retried. No password or session token is printed to stdout/stderr.
	if e = writeCredential(secretDir, *filename, name, password); e != nil {
		return e
	}
	if *action == "bootstrap" {
		_, e = models.BootstrapPortalAdmin(db, password, time.Now().UTC())
	} else {
		e = models.RecoverPortalAdmin(db, name, password, time.Now().UTC())
	}
	if e != nil {
		return errors.New("operation not confirmed; private credential file retained; inspect account/audit state before retry or recovery")
	}
	fmt.Println("Account operation confirmed. Temporary credentials are in the private operator file; no secret is printed. First-login password change is required.")
	return nil
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e.Error())
		os.Exit(1)
	}
}
