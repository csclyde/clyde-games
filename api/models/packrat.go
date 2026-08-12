package models

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

const packratLatestVersionSetting = "packrat_latest_version"
const defaultPackratVersion = "0.1.0"
const packratRepositoryManifest = "/root/Packrat/build.zig.zon"

var packratVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`)

type PackratVersion struct {
	Version string `gorm:"primaryKey;type:varchar(32)"`
}

type PackratVersionSettings struct {
	LatestVersion    string
	ObservedVersions []string
}

type PackratVersionCheck struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
}

func GetPackratVersionSettings() (*PackratVersionSettings, error) {
	if err := EnsureDefaultPackratVersion(); err != nil {
		return nil, err
	}
	if err := ObservePackratRepositoryVersion(); err != nil {
		return nil, err
	}

	latest, err := packratLatestVersion()
	if err != nil {
		return nil, err
	}
	observed, err := SelectPackratVersions()
	if err != nil {
		return nil, err
	}

	return &PackratVersionSettings{LatestVersion: latest, ObservedVersions: observed}, nil
}

func CheckPackratVersion(current string) (*PackratVersionCheck, error) {
	if err := EnsureDefaultPackratVersion(); err != nil {
		return nil, err
	}
	current = strings.TrimSpace(current)
	if current != "" {
		if err := SaveObservedPackratVersion(current); err != nil {
			return nil, err
		}
	}

	latest, err := packratLatestVersion()
	if err != nil {
		return nil, err
	}
	updateAvailable := false
	if current != "" && latest != "" {
		updateAvailable = compareSemanticVersions(current, latest) < 0
	}

	return &PackratVersionCheck{CurrentVersion: current, LatestVersion: latest, UpdateAvailable: updateAvailable}, nil
}

func SavePackratLatestVersion(version string) (*PackratVersionSettings, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		version = defaultPackratVersion
	}
	if !validPackratVersion(version) {
		return nil, errors.New("latest version must look like a semantic version, such as 0.1.0")
	}

	if err := EnsureDefaultPackratVersion(); err != nil {
		return nil, err
	}
	var count int64
	if err := AnalyticsDB.Model(&PackratVersion{}).Where("version = ?", version).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, errors.New("latest version must be one of the observed Packrat versions")
	}

	setting := EventSetting{Key: packratLatestVersionSetting}
	result := AnalyticsDB.Where("`key` = ?", packratLatestVersionSetting).FirstOrCreate(&setting)
	if result.Error != nil {
		return nil, result.Error
	}
	setting.Value = version
	if err := AnalyticsDB.Save(&setting).Error; err != nil {
		return nil, err
	}

	return GetPackratVersionSettings()
}

func EnsureDefaultPackratVersion() error {
	if err := SaveObservedPackratVersion(defaultPackratVersion); err != nil {
		return err
	}
	_, err := packratLatestVersion()
	return err
}

func SaveObservedPackratVersion(version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil
	}
	if !validPackratVersion(version) {
		return fmt.Errorf("invalid Packrat version %q", version)
	}
	return AnalyticsDB.FirstOrCreate(&PackratVersion{Version: version}, "version = ?", version).Error
}

func SelectPackratVersions() ([]string, error) {
	var versions []PackratVersion
	if err := AnalyticsDB.Find(&versions).Error; err != nil {
		return nil, err
	}

	values := make([]string, 0, len(versions))
	for _, version := range versions {
		values = append(values, version.Version)
	}
	sort.Slice(values, func(i, j int) bool {
		return compareSemanticVersions(values[i], values[j]) > 0
	})
	return values, nil
}

func ObservePackratRepositoryVersion() error {
	manifest, err := os.ReadFile(packratRepositoryManifest)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	const marker = ".version = \""
	text := string(manifest)
	start := strings.Index(text, marker)
	if start < 0 {
		return nil
	}
	versionText := text[start+len(marker):]
	end := strings.Index(versionText, "\"")
	if end < 0 {
		return nil
	}
	return SaveObservedPackratVersion(versionText[:end])
}

func packratLatestVersion() (string, error) {
	var setting EventSetting
	result := AnalyticsDB.Where("`key` = ?", packratLatestVersionSetting).First(&setting)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		setting = EventSetting{Key: packratLatestVersionSetting, Value: defaultPackratVersion}
		if err := AnalyticsDB.Create(&setting).Error; err != nil {
			return "", err
		}
		return defaultPackratVersion, nil
	}
	if result.Error != nil {
		return "", result.Error
	}
	if setting.Value == "" {
		return defaultPackratVersion, nil
	}
	return setting.Value, nil
}

func validPackratVersion(version string) bool {
	return packratVersionPattern.MatchString(version)
}

func compareSemanticVersions(left string, right string) int {
	leftParts := versionParts(left)
	rightParts := versionParts(right)
	for i := 0; i < 3; i++ {
		if leftParts[i] < rightParts[i] {
			return -1
		}
		if leftParts[i] > rightParts[i] {
			return 1
		}
	}
	leftPrerelease := prereleaseVersion(left)
	rightPrerelease := prereleaseVersion(right)
	if leftPrerelease == "" && rightPrerelease == "" {
		return 0
	}
	if leftPrerelease == "" {
		return 1
	}
	if rightPrerelease == "" {
		return -1
	}
	return strings.Compare(leftPrerelease, rightPrerelease)
}

func versionParts(version string) [3]int {
	var parts [3]int
	core := version
	if index := strings.IndexAny(core, "-+"); index >= 0 {
		core = core[:index]
	}
	split := strings.Split(core, ".")
	for index := 0; index < len(split) && index < 3; index++ {
		parts[index], _ = strconv.Atoi(split[index])
	}
	return parts
}

func prereleaseVersion(version string) string {
	start := strings.Index(version, "-")
	if start < 0 {
		return ""
	}
	value := version[start+1:]
	if end := strings.Index(value, "+"); end >= 0 {
		value = value[:end]
	}
	return value
}
