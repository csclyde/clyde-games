package models

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Crash struct {
	gorm.Model
	PID      string `gorm:"type:tinytext"`
	Project  string `gorm:"type:tinytext"`
	Message  string `gorm:"type:text"`
	Stack    string `gorm:"type:text"`
	Env      string `gorm:"type:tinytext"`
	Category string `gorm:"type:tinytext"`
	Platform string `gorm:"type:tinytext"`
	Build    string `gorm:"type:tinytext"`
	Commit   string `gorm:"type:tinytext"`
	Hash     string `gorm:"type:tinytext"`
	Count    uint   `gorm:"type:int"`
	Resolved bool   `gorm:"type:boolean;default:false"`
	// Version 0 rows are original, possibly aggregated history. Their Count,
	// metadata and timestamps must never be expanded into invented occurrences.
	OccurrenceVersion  uint `gorm:"not null;default:0"`
	IngestedAt         *time.Time
	ResolvedAt         *time.Time
	GroupKey           string  `gorm:"type:varchar(67);index"`
	GroupIdentity      string  `gorm:"type:longtext"`
	ReceiptKey         *string `gorm:"type:char(64);uniqueIndex" json:"-"`
	ClosureDisposition string  `gorm:"type:varchar(16)" json:",omitempty"`
}

const CrashReportIDHeader = "X-Clyde-Crash-Report-ID"

var ErrInvalidCrashReportID = errors.New("crash report ID requires 32 lowercase hexadecimal characters and a nonempty PID")
var hex32Pattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// CrashGroup stores identity/locking only; reports remain in crashes.
type CrashGroup struct {
	Key string `gorm:"primaryKey;type:varchar(67)"`
}

func lockCrashGroup(tx *gorm.DB, key string) error {
	group := CrashGroup{Key: key}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&group).Error; err != nil {
		return err
	}
	group = CrashGroup{}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("`key` = ?", key).Take(&group).Error
}

// MigrateCrashGroups is resumable and additive. Never change original hashes,
// counts, resolution, metadata or timestamps. A legacy UpdatedAt may be a
// resolution date, so it cannot truthfully become an occurrence timestamp.
func MigrateCrashGroups() error {
	for {
		var batch []Crash
		if err := AnalyticsDB.Unscoped().Where("group_key IS NULL OR group_key = ''").Order("id ASC").Limit(250).Find(&batch).Error; err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		for _, c := range batch {
			key, identity := CrashIdentityV1(c)
			if err := AnalyticsDB.Unscoped().Model(&Crash{}).Where("id = ? AND (group_key IS NULL OR group_key = '')", c.ID).UpdateColumns(map[string]interface{}{"group_key": key, "group_identity": identity}).Error; err != nil {
				return err
			}
		}
	}
}

type CrashBuild struct {
	Build   string
	Project string
	Count   int64
}

type CrashSettings struct {
	OldestBuild string
}

func SelectAllCrash() ([]Crash, error) {
	var allCrash []Crash
	result := AnalyticsDB.Where("resolved != true").Order("created_at desc, id desc").Find(&allCrash)

	if result.Error != nil {
		return nil, result.Error
	}

	return allCrash, nil
}

func SelectAccessViolationCrashesSince(since time.Time) ([]Crash, error) {
	var crashes []Crash
	result := AnalyticsDB.
		Where("message LIKE ?", "%Access Violation%").
		Where("created_at >= ?", since).
		Order("created_at desc, id desc").
		Find(&crashes)

	if result.Error != nil {
		return nil, result.Error
	}

	return crashes, nil
}

func SelectCrashBuilds(projects ...string) ([]CrashBuild, error) {
	var builds []CrashBuild
	query := AnalyticsDB.Model(&Crash{})
	if len(projects) > 0 {
		query = query.Where("project = ?", crashSettingsProject(projects))
	}
	result := query.
		Select("build, project, count(*) as count").
		Where("build <> ''").
		Group("build, project").
		Order("build desc, project asc").
		Find(&builds)

	if result.Error != nil {
		return nil, result.Error
	}

	return builds, nil
}

func GetCrashSettings(projects ...string) (*CrashSettings, error) {
	var setting EventSetting
	result := AnalyticsDB.Where("`key` = ?", projectCrashSettingKey(crashSettingsProject(projects))).First(&setting)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return &CrashSettings{}, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}

	return &CrashSettings{OldestBuild: setting.Value}, nil
}

func SaveOldestCrashBuild(build string, projects ...string) (*CrashSettings, error) {
	scope := crashSettingsProject(projects)
	if scope == "all" {
		return nil, errors.New("choose one project")
	}
	if scope != "" {
		p, e := SelectProject(scope)
		if e != nil {
			return nil, e
		}
		if p == nil {
			return nil, errors.New("project not found")
		}
	}
	if build != "" {
		if _, err := parseBuildDate(build); err != nil {
			return nil, fmt.Errorf("oldest build is not a valid date: %w", err)
		}

		var crashCount int64
		query := AnalyticsDB.Model(&Crash{}).Where("build = ?", build)
		if len(projects) > 0 {
			query = query.Where("project = ?", crashSettingsProject(projects))
		}
		if err := query.Count(&crashCount).Error; err != nil {
			return nil, err
		}
		if crashCount == 0 {
			return nil, errors.New("oldest build must be an existing crash build")
		}
	}

	setting := EventSetting{Key: projectCrashSettingKey(crashSettingsProject(projects))}
	result := AnalyticsDB.Where("`key` = ?", setting.Key).FirstOrCreate(&setting)
	if result.Error != nil {
		return nil, result.Error
	}
	setting.Value = build
	if err := AnalyticsDB.Save(&setting).Error; err != nil {
		return nil, err
	}

	return &CrashSettings{OldestBuild: build}, nil
}

func AddCrash(crash Crash) (*Crash, error) {
	return AddCrashWithReportID(crash, nil)
}

func AddCrashWithReportID(crash Crash, reportID *string) (*Crash, error) {
	crash.Project = NormalizeProjectName(crash.Project)
	crash.ClosureDisposition = ""
	crash.ReceiptKey = nil
	if crash.Project == FarmBuilderProject && reportID != nil {
		if !hex32Pattern.MatchString(*reportID) || strings.TrimSpace(crash.PID) == "" {
			return nil, ErrInvalidCrashReportID
		}
		identity, _ := json.Marshal([]string{crash.Project, crash.PID, *reportID})
		key := fmt.Sprintf("%x", sha256.Sum256(identity))
		crash.ReceiptKey = &key
	}

	// Discard all client-controlled persistence and resolution fields.
	crash.Model = gorm.Model{}
	crash.Count, crash.Resolved, crash.ResolvedAt = 1, false, nil
	crash.OccurrenceVersion = 1
	crash.IngestedAt = nil
	crash.GroupKey, crash.GroupIdentity = CrashIdentityV1(crash)
	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		if crash.ReceiptKey != nil {
			var existing Crash
			err := tx.Unscoped().Where("receipt_key = ?", *crash.ReceiptKey).Take(&existing).Error
			if err == nil {
				crash = existing
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if err := ensureProject(tx, crash.Project); err != nil {
			return err
		}
		if err := rejectBuildBeforeSetting(tx, projectCrashSettingKey(crash.Project), crash.Build, "crash"); err != nil {
			return err
		}
		if err := lockCrashGroup(tx, crash.GroupKey); err != nil {
			return err
		}
		now := tx.NowFunc()
		crash.CreatedAt, crash.UpdatedAt, crash.IngestedAt = now, now, &now
		if crash.ReceiptKey == nil {
			return tx.Omit("ReceiptKey", "ClosureDisposition").Create(&crash).Error
		}
		key := *crash.ReceiptKey
		if err := tx.Omit("ClosureDisposition").Clauses(clause.OnConflict{DoNothing: true}).Create(&crash).Error; err != nil {
			return err
		}
		// Use a current read so retries can see a simultaneous insert even
		// if the initial lookup used an earlier transaction snapshot.
		crash = Crash{}
		return tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("receipt_key = ?", key).Take(&crash).Error
	})
	if err != nil {
		return nil, err
	}

	return &crash, nil
}

func ResolveCrash(hash string, project ...string) (*Crash, error) {
	// Legacy hash callers remain supported when the hash identifies one group.
	// A collision or cross-project hash must never silently close other causes.
	var matches []Crash
	query := AnalyticsDB.Select("id", "group_key").Where("hash = ?", hash)
	if len(project) > 0 && project[0] != "" {
		query = query.Where("project = ?", NormalizeProjectName(project[0]))
	}
	if err := query.Find(&matches).Error; err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}
	for _, c := range matches {
		if c.GroupKey != matches[0].GroupKey {
			return nil, errors.New("ambiguous crash hash; resolve by report id")
		}
	}
	return ResolveCrashID(AnalyticsDB, uint64(matches[0].ID))
}

func ResolveCrashID(db *gorm.DB, id uint64) (*Crash, error) {
	return closeCrashID(db, id, false)
}

// IgnoreCrashID records dismissal through the same group closure as Resolve.
// Existing occurrences remain available for investigation and history.
func IgnoreCrashID(db *gorm.DB, id uint64) (*Crash, error) {
	return closeCrashID(db, id, true)
}

func closeCrashID(db *gorm.DB, id uint64, ignore bool) (*Crash, error) {
	var item Crash
	err := db.Transaction(func(tx *gorm.DB) error {
		// Read identity before taking the group lock. Taking member locks first
		// would deadlock simultaneous resolves addressed through different IDs.
		if err := tx.First(&item, id).Error; err != nil {
			return err
		}
		if item.GroupKey == "" {
			return errors.New("crash group migration required")
		}
		if err := lockCrashGroup(tx, item.GroupKey); err != nil {
			return err
		}
		now := tx.NowFunc()
		updates := map[string]interface{}{"resolved": true, "resolved_at": now}
		if ignore {
			updates["closure_disposition"] = "ignored"
		}
		if err := tx.Model(&Crash{}).Where("group_key = ? AND resolved = ?", item.GroupKey, false).UpdateColumns(updates).Error; err != nil {
			return err
		}
		// Refresh so repeated resolution returns its original resolution time.
		item = Crash{}
		return tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, id).Error
	})
	return &item, err
}

func ResolveAllCrashes() error {
	result := AnalyticsDB.Model(&Crash{}).Where("resolved != true").UpdateColumns(map[string]interface{}{"resolved": true, "resolved_at": AnalyticsDB.NowFunc()})
	return result.Error
}
