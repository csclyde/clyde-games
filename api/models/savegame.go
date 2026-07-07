package models

import "gorm.io/gorm"

type Savegame struct {
	gorm.Model
	PID         string `gorm:"type:tinytext"`
	SID         string `gorm:"type:tinytext"`
	Project     string `gorm:"type:tinytext"`
	Env         string `gorm:"type:tinytext"`
	Category    string `gorm:"type:tinytext"`
	Platform    string `gorm:"type:tinytext"`
	Build       string `gorm:"type:tinytext"`
	Commit      string `gorm:"type:tinytext"`
	Reason      string `gorm:"type:tinytext"`
	Filename    string `gorm:"type:tinytext"`
	ContentType string `gorm:"type:tinytext"`
	Hash        string `gorm:"type:char(64);index"`
	Size        uint32 `gorm:"type:int unsigned"`
	Data        []byte `gorm:"type:mediumblob;not null"`
}

type SavegameBuild struct {
	Build   string
	Project string
	Count   int64
}

func AddSavegame(savegame Savegame) (*Savegame, error) {
	result := AnalyticsDB.Create(&savegame)
	if result.Error != nil {
		return nil, result.Error
	}

	return &savegame, nil
}

func SelectAllSavegames() ([]Savegame, error) {
	var savegames []Savegame
	result := AnalyticsDB.Omit("Data").Order("created_at desc").Find(&savegames)
	if result.Error != nil {
		return nil, result.Error
	}

	return savegames, nil
}

func SelectSavegameBuilds() ([]SavegameBuild, error) {
	var builds []SavegameBuild
	result := AnalyticsDB.Model(&Savegame{}).
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

func SelectSavegame(id string) (*Savegame, error) {
	var savegame Savegame
	result := AnalyticsDB.Where("id = ?", id).First(&savegame)
	if result.Error != nil {
		return nil, result.Error
	}

	return &savegame, nil
}

func DeleteSavegame(id string) error {
	return AnalyticsDB.Delete(&Savegame{}, id).Error
}

func DeleteSavegamesByBuild(build string, project string) (int64, error) {
	query := AnalyticsDB.Unscoped().Where("build = ?", build)
	if project != "" {
		query = query.Where("project = ?", project)
	}

	result := query.Delete(&Savegame{})
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}
