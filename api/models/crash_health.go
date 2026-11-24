package models

import (
	"errors"
	"gorm.io/gorm"
	"sort"
	"strconv"
	"strings"
	"time"
)

const CrashHealthLimit = 100000

var ErrCrashHealthLimit = errors.New("crash health exceeds 100000 stored records; choose one project")

// No raw stack, PID list or legacy timestamp is returned by the health endpoint.
type CrashHealthRecord struct {
	ID                            uint
	Project, GroupKey, PID, Build string
	Count                         uint
	OccurrenceVersion             uint
	IngestedAt, ResolvedAt        *time.Time
	Resolved, AccessViolation     bool
}
type CrashHealthStats struct {
	Project              string     `json:"project"`
	GroupKey             string     `json:"group_key,omitempty"`
	ReportID             uint       `json:"report_id,omitempty"`
	Open                 bool       `json:"open"`
	Reports              int        `json:"reports"`
	ReportingIDs         int        `json:"reporting_ids"`
	MissingPIDReports    int        `json:"missing_pid_reports"`
	DetailedAllTime      int        `json:"detailed_all_time"`
	OpenDetailed         int        `json:"open_detailed"`
	UntimedDetailed      int        `json:"untimed_detailed"`
	LegacyRows           int        `json:"legacy_rows"`
	LegacyOccurrences    uint64     `json:"legacy_occurrences"`
	FirstReceived        *time.Time `json:"first_received"`
	LastReceived         *time.Time `json:"last_received"`
	LastResolution       *time.Time `json:"last_resolution"`
	ReturnedOpen         bool       `json:"returned_open"`
	ReturnedOpenGroups   int        `json:"returned_open_groups"`
	ReturnedReports      int        `json:"returned_reports"`
	ReturnedReportingIDs int        `json:"returned_reporting_ids"`
}
type CrashBuildHealth struct {
	Project           string     `json:"project"`
	Build             string     `json:"build"`
	Reports           int        `json:"reports"`
	ReportingIDs      int        `json:"reporting_ids"`
	MissingPIDReports int        `json:"missing_pid_reports"`
	Groups            int        `json:"groups"`
	OpenReports       int        `json:"open_reports"`
	ReturnedReports   int        `json:"returned_reports"`
	FirstReceived     *time.Time `json:"first_received"`
	LastReceived      *time.Time `json:"last_received"`
}
type CrashDayHealth struct {
	Day          string `json:"day"`
	Reports      int    `json:"reports"`
	ReportingIDs int    `json:"reporting_ids"`
}
type CrashHealth struct {
	Project              string             `json:"project"`
	Days                 int                `json:"days"`
	From                 time.Time          `json:"from"`
	To                   time.Time          `json:"to"`
	Complete             bool               `json:"complete"`
	StoredRows           int                `json:"stored_rows"`
	Reports              int                `json:"reports"`
	ReportingIDs         int                `json:"reporting_ids"`
	MissingPIDReports    int                `json:"missing_pid_reports"`
	DetailedAllTime      int                `json:"detailed_all_time"`
	UntimedDetailed      int                `json:"untimed_detailed"`
	LegacyRows           int                `json:"legacy_rows"`
	LegacyOccurrences    uint64             `json:"legacy_occurrences"`
	OpenGroups           int                `json:"open_groups"`
	ReturnedOpenGroups   int                `json:"returned_open_groups"`
	ReturnedReports      int                `json:"returned_reports"`
	ReturnedReportingIDs int                `json:"returned_reporting_ids"`
	FirstReceived        *time.Time         `json:"first_received"`
	LastReceived         *time.Time         `json:"last_received"`
	DaysData             []CrashDayHealth   `json:"daily"`
	Builds               []CrashBuildHealth `json:"builds"`
	Groups               []CrashHealthStats `json:"groups"`
	Families             []CrashHealthStats `json:"families"`
}
type healthAccumulator struct {
	stats                            CrashHealthStats
	ids, returnedIDs, returnedGroups map[string]bool
}

func newHealthAccumulator(project, key string, id uint) *healthAccumulator {
	return &healthAccumulator{stats: CrashHealthStats{Project: project, GroupKey: key, ReportID: id}, ids: map[string]bool{}, returnedIDs: map[string]bool{}, returnedGroups: map[string]bool{}}
}
func crashHealthKey(r CrashHealthRecord) string {
	key := r.GroupKey
	if key == "" {
		key = "report:" + strconv.FormatUint(uint64(r.ID), 10)
	}
	return r.Project + "\x00" + key
}
func healthReceived(t *time.Time, now time.Time) bool {
	return t != nil && !t.IsZero() && !t.After(now)
}
func healthTimes(first, last **time.Time, t *time.Time) {
	if *first == nil || t.Before(**first) {
		x := t.UTC()
		*first = &x
	}
	if *last == nil || t.After(**last) {
		x := t.UTC()
		*last = &x
	}
}
func healthPID(r CrashHealthRecord) string {
	p := strings.TrimSpace(r.PID)
	if p == "" {
		return ""
	}
	return r.Project + "\x00" + p
}
func (a *healthAccumulator) add(r CrashHealthRecord, from, now time.Time) {
	if !r.Resolved {
		a.stats.Open = true
	}
	if healthReceived(r.ResolvedAt, now) && (a.stats.LastResolution == nil || r.ResolvedAt.After(*a.stats.LastResolution)) {
		x := r.ResolvedAt.UTC()
		a.stats.LastResolution = &x
	}
	if r.OccurrenceVersion != 1 {
		a.stats.LegacyRows++
		a.stats.LegacyOccurrences += uint64(r.Count)
		return
	}
	a.stats.DetailedAllTime++
	if !r.Resolved {
		a.stats.OpenDetailed++
	}
	if !healthReceived(r.IngestedAt, now) {
		a.stats.UntimedDetailed++
		return
	}
	healthTimes(&a.stats.FirstReceived, &a.stats.LastReceived, r.IngestedAt)
	if r.IngestedAt.Before(from) {
		return
	}
	a.stats.Reports++
	if pid := healthPID(r); pid != "" {
		a.ids[pid] = true
	} else {
		a.stats.MissingPIDReports++
	}
}
func (a *healthAccumulator) finish() CrashHealthStats {
	s := a.stats
	s.ReportingIDs = len(a.ids)
	s.ReturnedReportingIDs = len(a.returnedIDs)
	s.ReturnedOpenGroups = len(a.returnedGroups)
	return s
}

func BuildCrashHealth(rows []CrashHealthRecord, project string, days int, now time.Time) *CrashHealth {
	now = now.UTC()
	from := now.Add(-time.Duration(days) * 24 * time.Hour)
	h := &CrashHealth{Project: project, Days: days, From: from, To: now, Complete: true, StoredRows: len(rows), DaysData: []CrashDayHealth{}, Builds: []CrashBuildHealth{}, Groups: []CrashHealthStats{}, Families: []CrashHealthStats{}}
	groups := map[string]*healthAccumulator{}
	families := map[string]*healthAccumulator{}
	total := newHealthAccumulator("", "", 0)
	type buildAcc struct {
		stats       CrashBuildHealth
		ids, groups map[string]bool
	}
	builds := map[string]*buildAcc{}
	type dayAcc struct {
		stats CrashDayHealth
		ids   map[string]bool
	}
	daily := map[string]*dayAcc{}
	for d := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC); !d.After(now); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		daily[key] = &dayAcc{stats: CrashDayHealth{Day: key}, ids: map[string]bool{}}
	}
	for _, r := range rows {
		r.Project = NormalizeProjectName(r.Project)
		key := crashHealthKey(r)
		if groups[key] == nil {
			groups[key] = newHealthAccumulator(r.Project, r.GroupKey, r.ID)
		}
		groups[key].add(r, from, now)
		total.add(r, from, now)
		if r.AccessViolation {
			if families[r.Project] == nil {
				families[r.Project] = newHealthAccumulator(r.Project, "", 0)
			}
			families[r.Project].add(r, from, now)
		}
		if r.OccurrenceVersion != 1 || !healthReceived(r.IngestedAt, now) || r.IngestedAt.Before(from) {
			continue
		}
		buildKey := r.Project + "\x00" + r.Build
		if builds[buildKey] == nil {
			builds[buildKey] = &buildAcc{stats: CrashBuildHealth{Project: r.Project, Build: r.Build}, ids: map[string]bool{}, groups: map[string]bool{}}
		}
		b := builds[buildKey]
		b.stats.Reports++
		b.groups[key] = true
		if !r.Resolved {
			b.stats.OpenReports++
		}
		healthTimes(&b.stats.FirstReceived, &b.stats.LastReceived, r.IngestedAt)
		day := daily[r.IngestedAt.UTC().Format("2006-01-02")]
		day.stats.Reports++
		if pid := healthPID(r); pid != "" {
			b.ids[pid] = true
			day.ids[pid] = true
		} else {
			b.stats.MissingPIDReports++
		}
	}
	// Only a known group resolution followed by a currently open detailed receipt
	// proves a returned issue. Legacy UpdatedAt/CreatedAt never substitutes for it.
	for _, r := range rows {
		r.Project = NormalizeProjectName(r.Project)
		key := crashHealthKey(r)
		g := groups[key]
		if r.Resolved || r.OccurrenceVersion != 1 || !healthReceived(r.IngestedAt, now) || g.stats.LastResolution == nil || !r.IngestedAt.After(*g.stats.LastResolution) {
			continue
		}
		g.stats.ReturnedOpen = true
		g.returnedGroups[key] = true
		total.returnedGroups[key] = true
		if r.AccessViolation {
			families[r.Project].stats.ReturnedOpen = true
			families[r.Project].returnedGroups[key] = true
		}
		if r.IngestedAt.Before(from) {
			continue
		}
		g.stats.ReturnedReports++
		total.stats.ReturnedReports++
		if pid := healthPID(r); pid != "" {
			g.returnedIDs[pid] = true
			total.returnedIDs[pid] = true
		}
		builds[r.Project+"\x00"+r.Build].stats.ReturnedReports++
		if r.AccessViolation {
			f := families[r.Project]
			f.stats.ReturnedReports++
			if pid := healthPID(r); pid != "" {
				f.returnedIDs[pid] = true
			}
		}
	}
	all := total.finish()
	h.Reports = all.Reports
	h.ReportingIDs = all.ReportingIDs
	h.MissingPIDReports = all.MissingPIDReports
	h.DetailedAllTime = all.DetailedAllTime
	h.UntimedDetailed = all.UntimedDetailed
	h.LegacyRows = all.LegacyRows
	h.LegacyOccurrences = all.LegacyOccurrences
	h.FirstReceived = all.FirstReceived
	h.LastReceived = all.LastReceived
	h.ReturnedOpenGroups = all.ReturnedOpenGroups
	h.ReturnedReports = all.ReturnedReports
	h.ReturnedReportingIDs = all.ReturnedReportingIDs
	for _, g := range groups {
		s := g.finish()
		if s.Open {
			h.OpenGroups++
		}
		h.Groups = append(h.Groups, s)
	}
	for _, f := range families {
		h.Families = append(h.Families, f.finish())
	}
	for _, b := range builds {
		b.stats.ReportingIDs = len(b.ids)
		b.stats.Groups = len(b.groups)
		h.Builds = append(h.Builds, b.stats)
	}
	for _, d := range daily {
		d.stats.ReportingIDs = len(d.ids)
		h.DaysData = append(h.DaysData, d.stats)
	}
	sort.Slice(h.DaysData, func(i, j int) bool { return h.DaysData[i].Day < h.DaysData[j].Day })
	sort.Slice(h.Builds, func(i, j int) bool {
		a, b := h.Builds[i], h.Builds[j]
		if a.Project != b.Project {
			return a.Project < b.Project
		}
		return a.Build > b.Build
	})
	sort.Slice(h.Groups, func(i, j int) bool {
		a, b := h.Groups[i], h.Groups[j]
		if a.Project != b.Project {
			return a.Project < b.Project
		}
		if a.GroupKey != b.GroupKey {
			return a.GroupKey < b.GroupKey
		}
		return a.ReportID < b.ReportID
	})
	sort.Slice(h.Families, func(i, j int) bool { return h.Families[i].Project < h.Families[j].Project })
	return h
}

func SelectCrashHealth(db *gorm.DB, project string, days int, now time.Time) (*CrashHealth, error) {
	rows := []CrashHealthRecord{}
	q := db.Model(&Crash{}).Select("id,project,group_key,p_id,build,count,occurrence_version,ingested_at,COALESCE(resolved,TRUE) AS resolved,resolved_at,LOWER(message) LIKE ? AS access_violation", "%access violation%")
	if project != "all" {
		q = q.Where("project = ?", NormalizeProjectID(project))
	}
	if e := q.Order("id").Limit(CrashHealthLimit + 1).Find(&rows).Error; e != nil {
		return nil, e
	}
	if len(rows) > CrashHealthLimit {
		return nil, ErrCrashHealthLimit
	}
	return BuildCrashHealth(rows, project, days, now), nil
}
