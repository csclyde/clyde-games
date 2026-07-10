package endpoints

import (
	"api.clyde.games/models"
	"bufio"
	"context"
	"errors"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type resourceUsage struct {
	Used    uint64  `json:"used"`
	Total   uint64  `json:"total"`
	Percent float64 `json:"percent"`
}

type systemStats struct {
	CPUPercent  float64       `json:"cpuPercent"`
	CPUCores    int           `json:"cpuCores"`
	Memory      resourceUsage `json:"memory"`
	Disk        resourceUsage `json:"disk"`
	Database    databaseStats `json:"database"`
	Uptime      uint64        `json:"uptime"`
	CollectedAt time.Time     `json:"collectedAt"`
}

type databaseStats struct {
	Healthy        bool   `json:"healthy"`
	Size           uint64 `json:"size"`
	ResponseTimeMS int64  `json:"responseTimeMs"`
	Connections    int    `json:"connections"`
}

type databaseResult struct {
	healthy        bool
	size           uint64
	responseTimeMS int64
}

type cpuTimes struct {
	idle  uint64
	total uint64
}

// GetSystemStats returns aggregate capacity information only. It deliberately
// omits hostnames, filesystem paths and process details from the public API.
func GetSystemStats(c *gin.Context) {
	memory, err := readMemoryUsage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read memory usage"})
		return
	}

	disk, err := readDiskUsage("/")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read disk usage"})
		return
	}

	cpuPercent, err := readCPUUsage(200 * time.Millisecond)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read CPU usage"})
		return
	}

	uptime, err := readUptime()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read uptime"})
		return
	}
	database := readDatabaseStats()

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, systemStats{
		CPUPercent:  cpuPercent,
		CPUCores:    runtime.NumCPU(),
		Memory:      memory,
		Disk:        disk,
		Database:    database,
		Uptime:      uptime,
		CollectedAt: time.Now().UTC(),
	})
}

func readDatabaseStats() databaseStats {
	databases := []*gorm.DB{models.AnalyticsDB, models.EtymologyDB}
	results := make(chan databaseResult, len(databases))

	for _, database := range databases {
		go func(db *gorm.DB) {
			results <- inspectDatabase(db)
		}(database)
	}

	stats := databaseStats{Healthy: true, Connections: len(databases)}
	for range databases {
		result := <-results
		stats.Healthy = stats.Healthy && result.healthy
		stats.Size += result.size
		if result.responseTimeMS > stats.ResponseTimeMS {
			stats.ResponseTimeMS = result.responseTimeMS
		}
	}
	return stats
}

func inspectDatabase(db *gorm.DB) databaseResult {
	if db == nil {
		return databaseResult{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	started := time.Now()

	sqlDB, err := db.DB()
	if err != nil || sqlDB.PingContext(ctx) != nil {
		return databaseResult{responseTimeMS: time.Since(started).Milliseconds()}
	}

	var size uint64
	err = db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(data_length + index_length), 0)
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
	`).Scan(&size).Error

	return databaseResult{
		healthy:        err == nil,
		size:           size,
		responseTimeMS: time.Since(started).Milliseconds(),
	}
}

func readMemoryUsage() (resourceUsage, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return resourceUsage{}, err
	}
	defer file.Close()

	var total, available uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total = value * 1024
		case "MemAvailable:":
			available = value * 1024
		}
	}
	if err := scanner.Err(); err != nil {
		return resourceUsage{}, err
	}
	if total == 0 || available > total {
		return resourceUsage{}, errors.New("invalid memory values")
	}

	return usage(total-available, total), nil
}

func readDiskUsage(path string) (resourceUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return resourceUsage{}, err
	}
	total := stat.Blocks * uint64(stat.Bsize)
	available := stat.Bavail * uint64(stat.Bsize)
	if total == 0 || available > total {
		return resourceUsage{}, errors.New("invalid disk values")
	}
	return usage(total-available, total), nil
}

func readCPUUsage(sample time.Duration) (float64, error) {
	start, err := readCPUTimes()
	if err != nil {
		return 0, err
	}
	time.Sleep(sample)
	end, err := readCPUTimes()
	if err != nil {
		return 0, err
	}

	totalDelta := end.total - start.total
	if totalDelta == 0 {
		return 0, nil
	}
	idleDelta := end.idle - start.idle
	return clampPercent(float64(totalDelta-idleDelta) / float64(totalDelta) * 100), nil
}

func readCPUTimes() (cpuTimes, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}, err
	}
	line := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuTimes{}, errors.New("invalid cpu values")
	}

	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return cpuTimes{}, err
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4] // iowait is idle time too
	}
	return cpuTimes{idle: idle, total: total}, nil
}

func readUptime() (uint64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, errors.New("invalid uptime")
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	return uint64(seconds), err
}

func usage(used, total uint64) resourceUsage {
	return resourceUsage{Used: used, Total: total, Percent: clampPercent(float64(used) / float64(total) * 100)}
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}
