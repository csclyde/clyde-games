package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}

		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func loadAvailableDotEnv() error {
	seen := map[string]bool{}
	paths := []string{".env"}

	if executablePath, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(executablePath), ".env"))
	}

	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true

		if err := loadDotEnv(path); err != nil {
			return err
		}
	}

	return nil
}
