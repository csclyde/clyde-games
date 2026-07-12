package models

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

const cursemarkProject = "cursemark"

type ProjectStats struct {
	Name      string `json:"name"`
	Crashes   int64  `json:"crashes"`
	Feedback  int64  `json:"feedback"`
	Savegames int64  `json:"savegames"`
	Events    int64  `json:"events"`
}

type projectCount struct {
	Project string
	Count   int64
}

func NormalizeProjectName(name string) string {
	name = strings.TrimSpace(name)
	if strings.EqualFold(name, cursemarkProject) {
		return cursemarkProject
	}
	return name
}

func SelectProjects() ([]ProjectStats, error) {
	projects := map[string]*ProjectStats{}
	queries := []struct {
		model interface{}
		apply func(*ProjectStats, int64)
		count string
	}{
		{&Crash{}, func(project *ProjectStats, count int64) { project.Crashes += count }, "COALESCE(SUM(`count`), 0)"},
		{&Feedback{}, func(project *ProjectStats, count int64) { project.Feedback += count }, "COUNT(*)"},
		{&Savegame{}, func(project *ProjectStats, count int64) { project.Savegames += count }, "COUNT(*)"},
		{&Event{}, func(project *ProjectStats, count int64) { project.Events += count }, "COUNT(*)"},
	}

	for _, query := range queries {
		var counts []projectCount
		if err := AnalyticsDB.Model(query.model).
			Select("project, " + query.count + " AS count").
			Where("TRIM(project) <> ''").Group("project").Find(&counts).Error; err != nil {
			return nil, err
		}
		for _, count := range counts {
			name := NormalizeProjectName(count.Project)
			key := strings.ToLower(name)
			project := projects[key]
			if project == nil {
				project = &ProjectStats{Name: name}
				projects[key] = project
			}
			query.apply(project, count.Count)
		}
	}

	result := make([]ProjectStats, 0, len(projects))
	for _, project := range projects {
		result = append(result, *project)
	}
	return result, nil
}

func RenameProject(oldName, newName string) error {
	oldName = NormalizeProjectName(oldName)
	newName = NormalizeProjectName(newName)
	if oldName == "" || newName == "" {
		return errors.New("old and new project names are required")
	}

	return AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		for _, model := range []interface{}{&Crash{}, &Feedback{}, &Savegame{}, &Event{}} {
			if err := tx.Unscoped().Model(model).
				Where("LOWER(TRIM(project)) = ?", strings.ToLower(oldName)).
				Update("project", newName).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func NormalizeKnownProjects() error {
	return RenameProject(cursemarkProject, cursemarkProject)
}
