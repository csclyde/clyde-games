package endpoints

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

const (
	maxSavegameBytes        = 1 << 20
	maxSavegameRequestBytes = maxSavegameBytes + 64*1024
)

var errSavegameTooLarge = errors.New("savegame exceeds 1MB limit")

type SavegameResponse struct {
	ID          uint
	CreatedAt   time.Time
	PID         string
	SID         string
	Project     string
	Env         string
	Category    string
	Platform    string
	Build       string
	Commit      string
	Reason      string
	Filename    string
	ContentType string
	Hash        string
	Size        uint32
}

func AddSavegame(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSavegameRequestBytes)

	savegame, err := readSavegame(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createdSavegame, err := models.AddSavegame(savegame)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusCreated, savegameResponse(*createdSavegame))
}

func GetSavegames(c *gin.Context) {
	savegames, err := models.SelectAllSavegames()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	responses := make([]SavegameResponse, 0, len(savegames))
	for _, savegame := range savegames {
		responses = append(responses, savegameResponse(savegame))
	}

	c.IndentedJSON(http.StatusOK, responses)
}

func GetSavegameBuilds(c *gin.Context) {
	builds, err := models.SelectSavegameBuilds()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, builds)
}

func DownloadSavegame(c *gin.Context) {
	savegame, err := models.SelectSavegame(c.Query("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	contentType := savegame.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	c.Header("Content-Disposition", `attachment; filename="test.sav"`)
	c.Data(http.StatusOK, contentType, savegame.Data)
}

func DeleteSavegame(c *gin.Context) {
	if err := models.DeleteSavegame(c.Query("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func DeleteSavegamesByBuild(c *gin.Context) {
	build := c.Query("build")
	if build == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "build is required"})
		return
	}

	deleted, err := models.DeleteSavegamesByBuild(build, c.Query("project"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

func readSavegame(c *gin.Context) (models.Savegame, error) {
	var savegame models.Savegame
	contentType := c.GetHeader("Content-Type")
	filename := c.Query("filename")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := c.Request.ParseMultipartForm(maxSavegameBytes); err != nil {
			return savegame, err
		}

		file, header, err := c.Request.FormFile("savegame")
		if err != nil {
			file, header, err = c.Request.FormFile("file")
		}
		if err != nil {
			return savegame, err
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			return savegame, err
		}

		filename = header.Filename
		contentType = header.Header.Get("Content-Type")
		savegame.Data = data
	} else {
		data, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return savegame, err
		}
		savegame.Data = data
	}

	if len(savegame.Data) == 0 {
		return savegame, http.ErrMissingFile
	}
	if len(savegame.Data) > maxSavegameBytes {
		return savegame, errSavegameTooLarge
	}

	hash := sha256.Sum256(savegame.Data)
	savegame.PID = requestValue(c, "pid")
	savegame.SID = requestValue(c, "sid")
	savegame.Project = requestValue(c, "project")
	savegame.Env = requestValue(c, "env")
	savegame.Category = requestValue(c, "category")
	savegame.Platform = requestValue(c, "platform")
	savegame.Build = requestValue(c, "build")
	savegame.Commit = requestValue(c, "commit")
	savegame.Reason = requestValue(c, "reason")
	savegame.Filename = filename
	savegame.ContentType = contentType
	savegame.Hash = hex.EncodeToString(hash[:])
	savegame.Size = uint32(len(savegame.Data))

	return savegame, nil
}

func requestValue(c *gin.Context, name string) string {
	if value := c.PostForm(name); value != "" {
		return value
	}

	return c.Query(name)
}

func savegameResponse(savegame models.Savegame) SavegameResponse {
	return SavegameResponse{
		ID:          savegame.ID,
		CreatedAt:   savegame.CreatedAt,
		PID:         savegame.PID,
		SID:         savegame.SID,
		Project:     savegame.Project,
		Env:         savegame.Env,
		Category:    savegame.Category,
		Platform:    savegame.Platform,
		Build:       savegame.Build,
		Commit:      savegame.Commit,
		Reason:      savegame.Reason,
		Filename:    savegame.Filename,
		ContentType: savegame.ContentType,
		Hash:        savegame.Hash,
		Size:        savegame.Size,
	}
}
