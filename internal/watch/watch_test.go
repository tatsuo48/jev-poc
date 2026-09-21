package watch

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tatsuo48/jev-poc/internal/game"
	"github.com/tatsuo48/jev-poc/internal/player"
	"github.com/tatsuo48/jev-poc/internal/runner"
)

func step(info player.Info) runner.Step {
	return runner.Step{
		Seed:   42,
		MoveNo: 17,
		After:  game.Board{{2048, 4, 0, 0}},
		Score:  1234,
		Move:   game.Left,
		Info:   info,
	}
}

func TestRenderShowsBoardAndStats(t *testing.T) {
	var out bytes.Buffer
	Render(&out, "greedy", step(player.Info{Latency: 3 * time.Millisecond}), Totals{})
	s := out.String()
	for _, want := range []string{"\x1b[H\x1b[2J", "player=greedy", "seed=42", "score=1234", "moves=17", "last=left", "latency=3ms", "2048"} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q:\n%q", want, s)
		}
	}
	if strings.Contains(s, "confidence") {
		t.Errorf("non-jev output shows confidence:\n%q", s)
	}
}

func TestRenderShowsProbabilitiesForJev(t *testing.T) {
	info := player.Info{
		Probabilities: map[game.Move]float64{game.Left: 0.75, game.Down: 0.25},
		Confidence:    0.5,
	}
	var out bytes.Buffer
	Render(&out, "jev-sim", step(info), Totals{InputTokens: 900, OutputTokens: 100})
	s := out.String()
	for _, want := range []string{"left  " + strings.Repeat("█", 15) + strings.Repeat("░", 5) + " 0.75", "down  ", "confidence=0.50", "tokens=1000"} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "up    ") {
		t.Errorf("output shows a move jev was not offered:\n%s", s)
	}
}

type firstLegal struct{}

func (firstLegal) Name() string { return "first-legal" }
func (firstLegal) Pick(_ context.Context, b game.Board) (game.Move, player.Info, error) {
	return game.LegalMoves(b)[0], player.Info{}, nil
}

type failing struct{}

func (failing) Name() string { return "failing" }
func (failing) Pick(context.Context, game.Board) (game.Move, player.Info, error) {
	return 0, player.Info{}, errors.New("boom")
}

func TestRunPlaysToTheEnd(t *testing.T) {
	var out bytes.Buffer
	res := Run(context.Background(), &out, firstLegal{}, 1, 0)
	if res.Err != nil || !strings.Contains(out.String(), "Game over") {
		t.Fatalf("res = %+v", res)
	}
}

func TestRunShowsTheError(t *testing.T) {
	var out bytes.Buffer
	res := Run(context.Background(), &out, failing{}, 1, 0)
	if res.Err == nil || !strings.Contains(out.String(), "error: boom") {
		t.Fatalf("res = %+v, out = %q", res, out.String())
	}
}
