package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tatsuo48/jev-poc/internal/game"
	"github.com/tatsuo48/jev-poc/internal/jev"
	"github.com/tatsuo48/jev-poc/internal/player"
	"github.com/tatsuo48/jev-poc/internal/runner"
)

type firstLegal struct{ name string }

func (f firstLegal) Name() string { return f.name }
func (f firstLegal) Pick(_ context.Context, b game.Board) (game.Move, player.Info, error) {
	info := player.Info{
		Probabilities: map[game.Move]float64{game.LegalMoves(b)[0]: 1},
		Confidence:    1,
		InputTokens:   10,
		OutputTokens:  1,
	}
	return game.LegalMoves(b)[0], info, nil
}

type failing struct {
	name string
	err  error
}

func (f failing) Name() string { return f.name }
func (f failing) Pick(context.Context, game.Board) (game.Move, player.Info, error) {
	return 0, player.Info{}, f.err
}

func factory(name string, _ int64) (player.Player, error) {
	switch name {
	case "a", "b":
		return firstLegal{name}, nil
	case "broken":
		return failing{name, errors.New("boom")}, nil
	case "broke":
		return failing{name, fmt.Errorf("wrapped: %w", jev.ErrBudgetExceeded)}, nil
	}
	return nil, fmt.Errorf("unknown player %q", name)
}

func TestRunPlaysEveryPlayerOnTheSameSeeds(t *testing.T) {
	results, err := Run(context.Background(), Config{Players: []string{"a", "b"}, Games: 3, Seed: 10, Parallel: 4, New: factory})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(results) != 6 {
		t.Fatalf("len = %d, want 6", len(results))
	}
	for i := 0; i < 3; i++ {
		a, b := results[i], results[i+3]
		if a.Player != "a" || b.Player != "b" || a.Seed != int64(10+i) || b.Seed != a.Seed {
			t.Fatalf("ordering: %+v / %+v", a, b)
		}
		if a.Score != b.Score || a.Moves != b.Moves {
			t.Fatalf("same strategy and seed differed: %+v / %+v", a, b)
		}
	}
}

func TestRunWritesOneJSONLinePerMove(t *testing.T) {
	var out bytes.Buffer
	results, err := Run(context.Background(), Config{Players: []string{"a"}, Games: 2, Seed: 1, Parallel: 2, New: factory, Out: &out})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	moves := results[0].Moves + results[1].Moves
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != moves {
		t.Fatalf("lines = %d, moves = %d", len(lines), moves)
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"player", "seed", "move_no", "board", "move", "probabilities", "confidence", "latency_ms"} {
		if _, ok := first[key]; !ok {
			t.Errorf("JSONL line lacks %q: %s", key, lines[0])
		}
	}
}

func TestSummarizeKeepsErroredGamesOutOfAverages(t *testing.T) {
	players := []string{"a", "broken"}
	results, err := Run(context.Background(), Config{Players: players, Games: 3, Seed: 1, Parallel: 2, New: factory})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	sums := Summarize(players, results)

	a, broken := sums[0], sums[1]
	if a.Player != "a" || a.Games != 3 || a.Errors != 0 || a.AvgScore <= 0 || a.BestScore < int(a.AvgScore) {
		t.Errorf("a = %+v", a)
	}
	tiles := 0
	for _, n := range a.MaxTiles {
		tiles += n
	}
	if tiles != 3 || a.Tokens == 0 {
		t.Errorf("a = %+v", a)
	}
	if broken.Games != 0 || broken.Errors != 3 || broken.AvgScore != 0 {
		t.Errorf("broken = %+v", broken)
	}

	var table bytes.Buffer
	WriteTable(&table, sums)
	if !strings.Contains(table.String(), "broken") || !strings.Contains(table.String(), "PLAYER") {
		t.Errorf("table:\n%s", table.String())
	}
}

// TestSummarizeCountsTokensAndLatencyOfAbortedGames covers a gap the
// existing tests miss: aborted games (Err != nil) must be excluded from
// score/move averages but must still contribute their tokens and latency,
// since those resources were genuinely spent (see runner.Result's doc).
func TestSummarizeCountsTokensAndLatencyOfAbortedGames(t *testing.T) {
	results := []runner.Result{
		{Player: "p", Score: 100, Moves: 10, MaxTile: 16, Latency: 10 * time.Millisecond, InputTokens: 50, OutputTokens: 5},
		{Player: "p", Score: 40, Moves: 4, Latency: 6 * time.Millisecond, InputTokens: 20, OutputTokens: 2, Err: errors.New("aborted")},
	}
	sums := Summarize([]string{"p"}, results)
	if len(sums) != 1 {
		t.Fatalf("len(sums) = %d, want 1", len(sums))
	}
	s := sums[0]
	if s.Games != 1 || s.Errors != 1 {
		t.Errorf("Games/Errors = %d/%d, want 1/1", s.Games, s.Errors)
	}
	if s.AvgScore != 100 || s.BestScore != 100 {
		t.Errorf("AvgScore/BestScore = %v/%v, want 100/100", s.AvgScore, s.BestScore)
	}
	if s.AvgMoves != 10 {
		t.Errorf("AvgMoves = %v, want 10", s.AvgMoves)
	}
	if len(s.MaxTiles) != 1 || s.MaxTiles[16] != 1 {
		t.Errorf("MaxTiles = %+v, want {16:1}", s.MaxTiles)
	}
	if s.Tokens != 77 {
		t.Errorf("Tokens = %d, want 77", s.Tokens)
	}
	wantLatency := 16 * time.Millisecond / 14
	if s.AvgLatency != wantLatency {
		t.Errorf("AvgLatency = %v, want %v", s.AvgLatency, wantLatency)
	}
}

func TestRunReportsUnknownPlayerAsError(t *testing.T) {
	results, err := Run(context.Background(), Config{Players: []string{"nobody"}, Games: 1, Seed: 1, Parallel: 1, New: factory})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(results) != 1 || results[0].Err == nil || results[0].Player != "nobody" {
		t.Fatalf("results = %+v", results)
	}
}

func TestBudgetExceededStopsTheRun(t *testing.T) {
	results, err := Run(context.Background(), Config{Players: []string{"broke", "a"}, Games: 50, Seed: 1, Parallel: 1, New: factory})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !BudgetExceeded(results) {
		t.Fatal("BudgetExceeded = false")
	}
	if len(results) >= 100 {
		t.Fatalf("run was not cut short: %d results", len(results))
	}
	if BudgetExceeded(nil) {
		t.Fatal("BudgetExceeded(nil) = true")
	}
}

type playerSeedCall struct {
	name string
	seed int64
}

func TestRunFeedsJobsSeedFirst(t *testing.T) {
	var (
		mu    sync.Mutex
		calls []playerSeedCall
	)
	newPlayer := func(name string, seed int64) (player.Player, error) {
		mu.Lock()
		calls = append(calls, playerSeedCall{name, seed})
		mu.Unlock()
		return firstLegal{name}, nil
	}
	_, err := Run(context.Background(), Config{Players: []string{"a", "b"}, Games: 2, Seed: 5, Parallel: 1, New: newPlayer})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := []playerSeedCall{{"a", 5}, {"b", 5}, {"a", 6}, {"b", 6}}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != len(want) {
		t.Fatalf("calls = %+v, want %+v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls = %+v, want %+v", calls, want)
		}
	}
}

type failingWriter struct {
	err error
}

func (w *failingWriter) Write(p []byte) (int, error) {
	return 0, w.err
}

func TestRunReportsMoveLogWriteError(t *testing.T) {
	writeErr := errors.New("write failed")
	failing := &failingWriter{err: writeErr}
	results, err := Run(context.Background(), Config{Players: []string{"a"}, Games: 2, Seed: 1, Parallel: 1, New: factory, Out: failing})
	if err == nil {
		t.Fatal("Run returned nil error, expected write error")
	}
	if !errors.Is(err, writeErr) {
		t.Fatalf("Run error does not match: got %v, want error with cause %v", err, writeErr)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("result[%d].Err = %v, want nil (game should still finish)", i, r.Err)
		}
		if r.Moves == 0 {
			t.Errorf("result[%d].Moves = 0, want > 0", i)
		}
	}
}
