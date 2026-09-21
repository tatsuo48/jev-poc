package main

import "testing"

// These tests call run([]string{...}) directly and assert the returned
// exit code. They never talk to the real jev API: TYPESAFE_API_KEY is left
// unset (or explicitly cleared with t.Setenv), and every case that would
// need a real key is expected to fail validation or the missing-key check
// before any HTTP request would be made.

func TestRunNoArgs(t *testing.T) {
	if code := run(nil); code != 2 {
		t.Fatalf("run(nil) = %d, want 2", code)
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	if code := run([]string{"frobnicate"}); code != 2 {
		t.Fatalf("run = %d, want 2", code)
	}
}

func TestRunBenchUnknownPlayer(t *testing.T) {
	if code := run([]string{"bench", "--players", "minimax"}); code != 2 {
		t.Fatalf("run = %d, want 2", code)
	}
}

func TestRunBenchTrimsPlayerNames(t *testing.T) {
	if code := run([]string{"bench", "--players", "random, greedy", "--games", "1"}); code != 0 {
		t.Fatalf("run = %d, want 0", code)
	}
}

func TestRunBenchDuplicatePlayer(t *testing.T) {
	if code := run([]string{"bench", "--players", "random,random"}); code != 2 {
		t.Fatalf("run = %d, want 2", code)
	}
}

func TestRunBenchZeroGames(t *testing.T) {
	if code := run([]string{"bench", "--games", "0"}); code != 2 {
		t.Fatalf("run = %d, want 2", code)
	}
}

func TestRunBenchHelp(t *testing.T) {
	if code := run([]string{"bench", "-h"}); code != 0 {
		t.Fatalf("run = %d, want 0", code)
	}
}

func TestRunWatchUnknownJevPlayer(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	if code := run([]string{"watch", "--player", "jev-typo"}); code != 2 {
		t.Fatalf("run = %d, want 2", code)
	}
}

func TestRunWatchJevPlayerWithoutKey(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	if code := run([]string{"watch", "--player", "jev-raw"}); code != 1 {
		t.Fatalf("run = %d, want 1", code)
	}
}

func TestRunWatchClassicPlayer(t *testing.T) {
	if code := run([]string{"watch", "--player", "greedy", "--seed", "3", "--delay", "0"}); code != 0 {
		t.Fatalf("run = %d, want 0", code)
	}
}
