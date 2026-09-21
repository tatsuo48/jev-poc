package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tatsuo48/jev-poc/internal/game"
	"github.com/tatsuo48/jev-poc/internal/jev"
	"github.com/tatsuo48/jev-poc/internal/player"
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
