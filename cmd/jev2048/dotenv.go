package main

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"
)

// dotEnvKeys are the only variables imported from a .env file.
var dotEnvKeys = map[string]bool{"TYPESAFE_API_KEY": true, "JEV_ENDPOINT": true}

// loadDotEnv imports KEY=VALUE lines for dotEnvKeys from path. Variables that
// are already set in the environment win, and a missing file is not an error.
// Values are never logged.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimPrefix(strings.TrimSpace(sc.Text()), "export ")
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !dotEnvKeys[key] || os.Getenv(key) != "" {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return sc.Err()
}
