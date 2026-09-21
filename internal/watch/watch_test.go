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
	// A classic player never asks jev, so a frame with zero cumulative
	// tokens must not show a tokens= field at all (it would otherwise
	// flicker between frames that did and didn't call the API).
	if strings.Contains(s, "tokens=") {
		t.Errorf("output with zero tokens shows a tokens= field:\n%q", s)
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
	for _, want := range []string{
		"left  " + strings.Repeat("█", 15) + strings.Repeat("░", 5) + " 0.75",
		"down  ",
		"score=1234  moves=17  last=left",
		"confidence=0.50",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "up    ") {
		t.Errorf("output shows a move jev was not offered:\n%s", s)
	}
	// The token count belongs on the second header line (with score/moves/
	// last/latency); the confidence line must be exactly "confidence=0.50"
	// with nothing appended.
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(line, "score="):
			if !strings.Contains(line, "tokens=1000") {
				t.Errorf("header line = %q, want it to contain tokens=1000", line)
			}
		case strings.HasPrefix(line, "confidence="):
			if line != "confidence=0.50" {
				t.Errorf("confidence line = %q, want exactly \"confidence=0.50\"", line)
			}
		}
	}
}

func TestRenderClampsOutOfRangeProbabilities(t *testing.T) {
	info := player.Info{
		Probabilities: map[game.Move]float64{game.Left: 1.7, game.Down: -0.2},
		Confidence:    0.5,
	}
	var out bytes.Buffer
	Render(&out, "jev-sim", step(info), Totals{}) // must not panic
	s := out.String()
	if !strings.Contains(s, "left  "+strings.Repeat("█", barWidth)+" 1.70") {
		t.Errorf("left bar not clamped to full:\n%s", s)
	}
	if !strings.Contains(s, "down  "+strings.Repeat("░", barWidth)+" -0.20") {
		t.Errorf("down bar not clamped to empty:\n%s", s)
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
