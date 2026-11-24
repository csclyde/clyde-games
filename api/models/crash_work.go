package models

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sort"
	"strings"
	"time"
)

var ErrCrashGroupRequired = errors.New("crash investigation belongs to its group; use group investigation routes")
var ErrCrashClaim = errors.New("active group investigation claim required; read current group before retry")
var ErrCrashClaimConflict = errors.New("group investigation is claimed, expired, completed or its lifecycle changed")

// One durable unit per canonical project and real identity, or per AV display
// family. No occurrence, historical note or closure flag is rewritten.
type CrashInvestigationUnit struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	Key                 string    `gorm:"type:char(64);uniqueIndex" json:"key"`
	Project             string    `gorm:"type:varchar(64)" json:"project"`
	GroupKey            string    `gorm:"type:varchar(67)" json:"group_key,omitempty"`
	Family              string    `gorm:"type:varchar(32)" json:"family,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	Version             uint
	CoveredThroughID    uint       `json:"covered_through_id"`
	CoveredRevision     string     `gorm:"type:char(64)"`
	CoveredResolutionAt *time.Time `json:"covered_resolution_at,omitempty"`
	LeaseToken          string     `gorm:"type:char(64)" json:"-"`
	LeaseUntil          *time.Time `json:"lease_until,omitempty"`
}

// Durable request identity survives lease expiration/restarts. An exact retry
// of a completed claim never starts a second investigation.
type CrashInvestigationClaim struct {
	ID           uint   `gorm:"primaryKey"`
	UnitID       uint   `gorm:"uniqueIndex:crash_claim_request,priority:1"`
	RequestKey   string `gorm:"type:varbinary(80);uniqueIndex:crash_claim_request,priority:2"`
	Token        string `gorm:"type:char(64)"`
	ExpiresAt    time.Time
	CompletedAt  *time.Time
	Version      uint
	ThroughID    uint
	ResolutionAt *time.Time
	Revision     string                  `gorm:"type:char(64)"`
	Unit         *CrashInvestigationUnit `gorm:"foreignKey:UnitID;constraint:OnDelete:RESTRICT" json:"-"`
}

func MigrateCrashInvestigations(db *gorm.DB) error {
	return db.AutoMigrate(&CrashInvestigationUnit{}, &CrashInvestigationClaim{})
}

type CrashWorkScope struct {
	Project, GroupKey, Family, Key string
	AnchorID                       uint
}

func ResolveCrashWorkScope(db *gorm.DB, id uint, project string) (CrashWorkScope, error) {
	s := CrashWorkScope{Project: project, AnchorID: id}
	if id == 0 || project != NormalizeProjectID(project) || len(project) > 64 {
		return s, ErrNoteInput
	}
	var a struct {
		ID                         uint
		Project, GroupKey, Message string
	}
	if e := db.Table("crashes").Select("id,project,group_key,message").Where("id=? AND deleted_at IS NULL", id).Take(&a).Error; e != nil {
		return s, e
	}
	if NormalizeProjectID(a.Project) != project {
		return s, gorm.ErrRecordNotFound
	}
	identity := "group:" + a.GroupKey
	s.GroupKey = a.GroupKey
	if strings.Contains(strings.ToLower(a.Message), "access violation") {
		s.Family = "access_violations"
		s.GroupKey = ""
		identity = "family:access_violations"
	} else if a.GroupKey == "" {
		identity = fmt.Sprintf("legacy-report:%d", id)
	}
	raw, _ := json.Marshal([]string{"crash-investigation-v1", project, s.Family, identity})
	hash := sha256.Sum256(raw)
	s.Key = hex.EncodeToString(hash[:])
	return s, nil
}
func (s CrashWorkScope) Reports(db *gorm.DB) *gorm.DB {
	q := db.Table("crashes AS c").Where("c.deleted_at IS NULL").Where("(BINARY COALESCE(TRIM(c.project),'')=? OR (?='cursemark' AND LOWER(TRIM(c.project))='cursemark'))", s.Project, s.Project)
	if s.Family != "" {
		return q.Where("LOWER(c.message) LIKE ?", "%access violation%")
	}
	if s.GroupKey != "" {
		return q.Where("BINARY c.group_key=?", s.GroupKey)
	}
	return q.Where("c.id=?", s.AnchorID)
}
func (s CrashWorkScope) Notes(db *gorm.DB, unitID uint) *gorm.DB {
	members := s.Reports(db).Select("c.id")
	if unitID == 0 {
		return db.Model(&ReportNote{}).Where("BINARY project=? AND kind='crash' AND report_id IN (?)", s.Project, members)
	}
	return db.Model(&ReportNote{}).Where("BINARY project=? AND ((kind='crash' AND report_id IN (?)) OR (kind='crashgrp' AND report_id=?))", s.Project, members, unitID)
}

type CrashWorkView struct {
	Key                 string     `json:"key"`
	Project             string     `json:"project"`
	GroupKey            string     `json:"group_key,omitempty"`
	Family              string     `json:"family,omitempty"`
	AnchorID            uint       `json:"anchor_id"`
	State               string     `json:"state"`
	ReleaseState        string     `json:"release_state,omitempty"`
	PreviousState       string     `json:"previous_state,omitempty"`
	NoteCount           int64      `json:"note_count"`
	LastNoteAt          *time.Time `json:"last_note_at,omitempty"`
	ReportCount         int64      `json:"report_count"`
	OpenReportCount     int64      `json:"open_report_count"`
	LatestReportID      uint       `json:"latest_report_id"`
	CoveredThroughID    uint       `json:"covered_through_id"`
	LastResolution      *time.Time `json:"last_resolution,omitempty"`
	Returned            bool       `json:"returned"`
	NeedsReview         bool       `json:"needs_review"`
	LeaseUntil          *time.Time `json:"lease_until,omitempty"`
	Revision            string     `json:"revision"`
	LegacyStateConflict bool       `json:"legacy_state_conflict,omitempty"`
}

func (s CrashWorkScope) View(db *gorm.DB, u CrashInvestigationUnit) (CrashWorkView, error) {
	v := CrashWorkView{Key: s.Key, Project: s.Project, GroupKey: s.GroupKey, Family: s.Family, AnchorID: s.AnchorID, State: "unreviewed", LeaseUntil: u.LeaseUntil, CoveredThroughID: u.CoveredThroughID}
	var agg struct {
		Reports, Open int64
		Latest        uint
		Resolution    *time.Time
	}
	if e := s.Reports(db).Select("COUNT(*) AS reports,COALESCE(SUM(c.resolved=0),0) AS open,COALESCE(MAX(c.id),0) AS latest,MAX(c.resolved_at) AS resolution").Scan(&agg).Error; e != nil {
		return v, e
	}
	v.ReportCount, v.OpenReportCount, v.LatestReportID, v.LastResolution = agg.Reports, agg.Open, agg.Latest, agg.Resolution
	q := s.Notes(db, u.ID)
	if e := q.Count(&v.NoteCount).Error; e != nil {
		return v, e
	}
	var latest, state, release ReportNote
	for _, entry := range []struct {
		dest      *ReportNote
		condition string
	}{{&latest, "1=1"}, {&state, "state<>''"}, {&release, "release_state<>''"}} {
		order := "id DESC"
		if entry.condition != "1=1" {
			order = "CASE WHEN kind='crashgrp' THEN 0 ELSE 1 END ASC,id DESC"
		}
		e := s.Notes(db, u.ID).Select("id,kind,created_at,state,release_state").Where(entry.condition).Order(order).Take(entry.dest).Error
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return v, e
		}
	}
	if latest.ID > 0 {
		v.LastNoteAt = &latest.CreatedAt
	}
	v.State = CanonicalInvestigationState(state.State)
	v.ReleaseState = release.ReleaseState
	// Legacy notes are retained, not copied. Infer only the actually annotated
	// report frontier, never silently acknowledge later arrivals.
	// Retained timing is compared per REAL incident, even inside the AV family.
	if v.ReportCount > 100000 {
		return v, ErrCrashHealthLimit
	}
	type life struct {
		ID                     uint
		GroupKey               string
		Resolved               bool
		OccurrenceVersion      uint
		ResolvedAt, IngestedAt *time.Time
	}
	lives := []life{}
	if e := s.Reports(db).Select("c.id,c.group_key,c.resolved,c.occurrence_version,c.resolved_at,c.ingested_at").Scan(&lives).Error; e != nil {
		return v, e
	}
	resolution := map[string]time.Time{}
	keyOf := func(r life) string {
		if r.GroupKey != "" {
			return r.GroupKey
		}
		return fmt.Sprintf("legacy:%d", r.ID)
	}
	for _, r := range lives {
		if r.ResolvedAt != nil && !r.ResolvedAt.IsZero() {
			k := keyOf(r)
			if r.ResolvedAt.After(resolution[k]) {
				resolution[k] = *r.ResolvedAt
			}
		}
	}
	keys := []string{}
	for k := range resolution {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	signature := [][2]string{}
	for _, k := range keys {
		signature = append(signature, [2]string{k, resolution[k].UTC().Format(time.RFC3339Nano)})
	}
	raw, _ := json.Marshal(signature)
	h := sha256.Sum256(raw)
	v.Revision = hex.EncodeToString(h[:])
	for _, r := range lives {
		t, ok := resolution[keyOf(r)]
		if ok && !r.Resolved && r.OccurrenceVersion == 1 && r.IngestedAt != nil && !r.IngestedAt.IsZero() && r.IngestedAt.After(t) {
			v.Returned = true
		}
	}
	coveredRevision := u.CoveredRevision
	if u.ID == 0 {
		if e := s.Notes(db, 0).Where("kind='crash'").Select("COALESCE(MAX(report_id),0)").Scan(&v.CoveredThroughID).Error; e != nil {
			return v, e
		}
		if agg.Resolution != nil && latest.ID > 0 && !latest.CreatedAt.Before(*agg.Resolution) {
			coveredRevision = v.Revision
		}
	}
	if state.Kind == "crash" {
		ids := s.Notes(db, u.ID).Where("kind='crash' AND state<>''").Select("MAX(id)").Group("report_id")
		var variants int64
		if e := db.Model(&ReportNote{}).Where("id IN (?)", ids).Select("COUNT(DISTINCT CASE WHEN state='fixed' THEN 'ready_for_review' ELSE state END)").Scan(&variants).Error; e != nil {
			return v, e
		}
		v.LegacyStateConflict = variants > 1
	}
	newCycle := v.Returned && coveredRevision != v.Revision

	v.NeedsReview = v.OpenReportCount > 0 && ((v.State == "unreviewed" || v.State == "investigating") || v.LatestReportID > v.CoveredThroughID || newCycle || v.LegacyStateConflict)
	if v.NeedsReview && (v.State == "ready_for_review" || v.LegacyStateConflict) {
		v.PreviousState = v.State
		v.State = "needs_confirmation"
	}
	return v, nil
}
func (s CrashWorkScope) Unit(db *gorm.DB, lock bool) (CrashInvestigationUnit, error) {
	u := CrashInvestigationUnit{}
	q := db.Where("`key`=?", s.Key)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	e := q.Take(&u).Error
	if errors.Is(e, gorm.ErrRecordNotFound) && !lock {
		return u, nil
	}
	return u, e
}
func (s CrashWorkScope) EnsureLocked(db *gorm.DB) (CrashInvestigationUnit, error) {
	current, e := s.Unit(db, false)
	if e != nil {
		return current, e
	}
	if current.ID == 0 {
		v, e := s.View(db, current)
		if e != nil {
			return current, e
		}
		current = CrashInvestigationUnit{Key: s.Key, Project: s.Project, GroupKey: s.GroupKey, Family: s.Family, CoveredThroughID: v.CoveredThroughID}
		if v.LastResolution != nil && v.LastNoteAt != nil && !v.LastNoteAt.Before(*v.LastResolution) {
			current.CoveredResolutionAt = v.LastResolution
			current.CoveredRevision = v.Revision
		}
		if e := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&current).Error; e != nil {
			return current, e
		}
	}
	return s.Unit(db, true)
}
func GetCrashWork(db *gorm.DB, id uint, project string) (CrashWorkView, error) {
	s, e := ResolveCrashWorkScope(db, id, project)
	if e != nil {
		return CrashWorkView{}, e
	}
	u, e := s.Unit(db, false)
	if e != nil {
		return CrashWorkView{}, e
	}
	return s.View(db, u)
}

type CrashClaimResult struct {
	Acquired      bool          `json:"acquired"`
	Reason        string        `json:"reason,omitempty"`
	Token         string        `json:"claim_token,omitempty"`
	ExpiresAt     *time.Time    `json:"expires_at,omitempty"`
	Investigation CrashWorkView `json:"investigation"`
}

func ClaimCrashWork(db *gorm.DB, id uint, project, key, renewToken string) (CrashClaimResult, error) {
	out := CrashClaimResult{}
	if e := ValidateReportNote(AppendReportNoteInput{Note: "claim", RequestKey: key}); e != nil {
		return out, e
	}
	s, e := ResolveCrashWorkScope(db, id, project)
	if e != nil {
		return out, e
	}
	e = db.Transaction(func(tx *gorm.DB) error {
		u, e := s.EnsureLocked(tx)
		if e != nil {
			return e
		}
		v, e := s.View(tx, u)
		if e != nil {
			return e
		}
		out.Investigation = v
		now := tx.NowFunc().UTC()
		claim := CrashInvestigationClaim{}
		e = tx.Where("unit_id=? AND request_key=?", u.ID, key).Take(&claim).Error
		if e == nil {
			if claim.CompletedAt != nil {
				out.Reason = "already_completed"
				return nil
			}
			if claim.Version != u.Version || v.Revision != claim.Revision || v.OpenReportCount == 0 {
				out.Reason = "investigation_changed"
				return nil
			}

			if !claim.ExpiresAt.After(now) || u.LeaseToken != claim.Token {
				out.Reason = "claim_expired"
				return nil
			}
			if renewToken != "" {
				if renewToken != claim.Token {
					return ErrCrashClaim
				}
				claim.ExpiresAt = now.Add(30 * time.Minute)
				if e := tx.Model(&claim).Update("expires_at", claim.ExpiresAt).Error; e != nil {
					return e
				}
				if e := tx.Model(&u).Update("lease_until", claim.ExpiresAt).Error; e != nil {
					return e
				}
			}
			out.Acquired = true
			out.Token = claim.Token
			out.ExpiresAt = &claim.ExpiresAt
			out.Investigation.LeaseUntil = &claim.ExpiresAt
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if renewToken != "" {
			return ErrCrashClaim
		}
		if u.LeaseUntil != nil && u.LeaseUntil.After(now) {
			out.Reason = "already_claimed"
			return nil
		}
		if !v.NeedsReview {
			out.Reason = "already_reviewed_or_closed"
			return nil
		}
		bytes := make([]byte, 32)
		if _, e := rand.Read(bytes); e != nil {
			return e
		}
		token := hex.EncodeToString(bytes)
		until := now.Add(30 * time.Minute)
		claim = CrashInvestigationClaim{UnitID: u.ID, RequestKey: key, Token: token, ExpiresAt: until, Version: u.Version, ThroughID: v.LatestReportID, ResolutionAt: v.LastResolution, Revision: v.Revision}
		if e := tx.Create(&claim).Error; e != nil {
			return e
		}
		if e := tx.Model(&u).Updates(map[string]any{"lease_token": token, "lease_until": until}).Error; e != nil {
			return e
		}
		out.Acquired = true
		out.Token = token
		out.ExpiresAt = &until
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, e
}

type AppendCrashWorkInput struct {
	RequestKey   string `json:"request_key"`
	Note         string `json:"note"`
	State        string `json:"state,omitempty"`
	ReleaseState string `json:"release_state,omitempty"`
	ClaimToken   string `json:"claim_token,omitempty"`
	Complete     bool   `json:"complete,omitempty"`
}

func AppendCrashWork(db *gorm.DB, id uint, project, actor string, in AppendCrashWorkInput) (ReportNote, bool, error) {
	n := ReportNote{}
	replay := false
	if actor != "admin" && actor != "mcp" {
		return n, false, ErrNoteInput
	}
	if e := ValidateReportNote(AppendReportNoteInput{RequestKey: in.RequestKey, Note: in.Note, State: in.State, ReleaseState: in.ReleaseState}); e != nil {
		return n, false, e
	}
	if actor == "mcp" && (in.ClaimToken == "" || (in.Complete && in.State == "")) {
		return n, false, ErrCrashClaim
	}
	if actor == "admin" && (in.ClaimToken != "" || in.Complete) {
		return n, false, ErrNoteInput
	}
	s, e := ResolveCrashWorkScope(db, id, project)
	if e != nil {
		return n, false, e
	}
	raw, _ := json.Marshal([]any{actor, in})
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	e = db.Transaction(func(tx *gorm.DB) error {
		u, e := s.EnsureLocked(tx)
		if e != nil {
			return e
		}
		e = tx.Where("kind='crashgrp' AND report_id=? AND request_key=?", u.ID, in.RequestKey).Take(&n).Error
		if e == nil {
			if n.PayloadHash != digest {
				return ErrNoteConflict
			}
			replay = true
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		var claim CrashInvestigationClaim
		if actor == "mcp" {
			now := tx.NowFunc().UTC()
			if u.LeaseToken != in.ClaimToken || u.LeaseUntil == nil || !u.LeaseUntil.After(now) {
				return ErrCrashClaim
			}
			if e := tx.Where("unit_id=? AND token=?", u.ID, in.ClaimToken).Take(&claim).Error; e != nil {
				return ErrCrashClaim
			}
			if claim.CompletedAt != nil || !claim.ExpiresAt.After(now) {
				return ErrCrashClaim
			}
			if claim.Version != u.Version {
				return ErrCrashClaimConflict
			}
			v, e := s.View(tx, u)
			if e != nil {
				return e
			}
			if v.OpenReportCount == 0 || v.Revision != claim.Revision {
				return ErrCrashClaimConflict
			}
		}
		n = ReportNote{Kind: "crashgrp", ReportID: u.ID, Project: project, Actor: actor, RequestKey: in.RequestKey, PayloadHash: digest, CreatedAt: tx.NowFunc().UTC(), Note: in.Note, State: in.State, ReleaseState: in.ReleaseState}
		if e := tx.Create(&n).Error; e != nil {
			return e
		}
		if actor == "mcp" && in.Complete {
			now := tx.NowFunc().UTC()
			if e := tx.Model(&claim).Update("completed_at", now).Error; e != nil {
				return e
			}
			return tx.Model(&u).Updates(map[string]any{"covered_through_id": claim.ThroughID, "covered_resolution_at": claim.ResolutionAt, "covered_revision": claim.Revision, "lease_token": "", "lease_until": nil, "version": u.Version + 1}).Error
		}
		if actor == "admin" {
			update := map[string]any{"version": u.Version + 1}
			if in.State != "" {
				v, e := s.View(tx, u)
				if e != nil {
					return e
				}
				update["covered_through_id"] = v.LatestReportID
				update["covered_resolution_at"] = v.LastResolution
				update["covered_revision"] = v.Revision
				update["lease_token"] = ""
				update["lease_until"] = nil
			}
			return tx.Model(&u).Updates(update).Error
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return n, replay, e
}
