package main

import (
	"api.clyde.games/models"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"sort"
	"strings"
)

// Offline restored-copy rehearsal only. No collector startup or credentials.
// Hash every original non-auth table, its schema and every row (duplicates
// included), without printing any player data. The copy must remain quiescent.
func preservedFingerprint(db *gorm.DB) (string, int, uint64, error) {
	pool, e := db.DB()
	if e != nil {
		return "", 0, 0, e
	}
	rs, e := pool.Query("SHOW FULL TABLES WHERE Table_type = 'BASE TABLE'")
	if e != nil {
		return "", 0, 0, e
	}
	var names []string
	for rs.Next() {
		var n, kind string
		if e = rs.Scan(&n, &kind); e != nil {
			rs.Close()
			return "", 0, 0, e
		}
		switch n {
		case "portal_users", "portal_sessions", "portal_auth_controls", "portal_auth_events":
		default:
			names = append(names, n)
		}
	}
	e = rs.Err()
	rs.Close()
	if e != nil {
		return "", 0, 0, e
	}
	sort.Strings(names)
	h := sha256.New()
	var count uint64
	for _, name := range names {
		quoted := "`" + strings.ReplaceAll(name, "`", "``") + "`"
		var table, ddl string
		if e = pool.QueryRow("SHOW CREATE TABLE "+quoted).Scan(&table, &ddl); e != nil {
			return "", 0, 0, e
		}
		fmt.Fprintf(h, "%d:%s%d:%s", len(name), name, len(ddl), ddl)
		rows, e := pool.Query("SELECT * FROM " + quoted)
		if e != nil {
			return "", 0, 0, e
		}
		cols, e := rows.Columns()
		if e != nil {
			rows.Close()
			return "", 0, 0, e
		}
		var hashes []string
		for rows.Next() {
			values := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if e = rows.Scan(ptrs...); e != nil {
				rows.Close()
				return "", 0, 0, e
			}
			rh := sha256.New()
			for _, v := range values {
				if v == nil {
					rh.Write([]byte{0})
					continue
				}
				rh.Write([]byte{1})
				var raw []byte
				if b, ok := v.([]byte); ok {
					raw = b
				} else {
					raw = []byte(fmt.Sprint(v))
				}
				var size [8]byte
				binary.BigEndian.PutUint64(size[:], uint64(len(raw)))
				rh.Write(size[:])
				rh.Write(raw)
			}
			hashes = append(hashes, hex.EncodeToString(rh.Sum(nil)))
			count++
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return "", 0, 0, e
		}
		sort.Strings(hashes)
		for _, s := range hashes {
			h.Write([]byte(s))
		}
	}
	return hex.EncodeToString(h.Sum(nil)), len(names), count, nil
}
func rehearse(db *gorm.DB, name string) error {
	if !strings.HasPrefix(name, "portal_auth_rehearsal_") {
		return errors.New("rehearsal requires a private restored copy named portal_auth_rehearsal_...")
	}
	before, tables, rows, e := preservedFingerprint(db)
	if e != nil {
		return errors.New("cannot fingerprint restored copy")
	}
	for i := 0; i < 2; i++ {
		if models.MigratePortalAuth(db) != nil {
			return errors.New("auth rehearsal migration failed")
		}
		after, n, r, e := preservedFingerprint(db)
		if e != nil || after != before || n != tables || r != rows {
			return errors.New("existing restored records or schema changed")
		}
	}
	for _, model := range []any{&models.PortalUser{}, &models.PortalSession{}, &models.PortalAuthEvent{}} {
		var n int64
		if db.Model(model).Count(&n).Error != nil || n != 0 {
			return errors.New("rehearsal must not contain accounts, sessions or auth events")
		}
	}
	var controls int64
	if db.Model(&models.PortalAuthControl{}).Count(&controls).Error != nil || controls != 1 {
		return errors.New("singleton migration is not idempotent")
	}
	fmt.Printf("Rehearsal passed twice: %d existing tables, %d rows, SHA256 %s; no users or credentials created.\n", tables, rows, before)
	return nil
}
