package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDotEnv(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDotEnvSetsKnownVariables(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	t.Setenv("JEV_ENDPOINT", "")
	path := writeDotEnv(t, "# comment\n\nexport TYPESAFE_API_KEY=\"from-file\"\nJEV_ENDPOINT = http://localhost:1/x\n")

	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TYPESAFE_API_KEY"); got != "from-file" {
		t.Errorf("TYPESAFE_API_KEY = %q, want from-file", got)
	}
	if got := os.Getenv("JEV_ENDPOINT"); got != "http://localhost:1/x" {
		t.Errorf("JEV_ENDPOINT = %q", got)
	}
}

func TestLoadDotEnvNeverOverridesTheEnvironment(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "from-env")
	if err := loadDotEnv(writeDotEnv(t, "TYPESAFE_API_KEY=from-file\n")); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TYPESAFE_API_KEY"); got != "from-env" {
		t.Errorf("TYPESAFE_API_KEY = %q, want from-env", got)
	}
}

func TestLoadDotEnvIgnoresOtherVariables(t *testing.T) {
	t.Setenv("JEV2048_UNRELATED", "")
	if err := loadDotEnv(writeDotEnv(t, "JEV2048_UNRELATED=x\nAPI_KEY=y\n")); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("JEV2048_UNRELATED"); got != "" {
		t.Errorf("an unrelated variable was imported: %q", got)
	}
}

func TestLoadDotEnvMissingFileIsNotAnError(t *testing.T) {
	if err := loadDotEnv(filepath.Join(t.TempDir(), "absent")); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}
