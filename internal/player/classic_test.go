package player

import (
	"context"
	"errors"
	"testing"

	"github.com/tatsuo48/jev-poc/internal/game"
)

var stuck = game.Board{
	{2, 4, 2, 4},
	{4, 2, 4, 2},
	{2, 4, 2, 4},
	{4, 2, 4, 2},
}

func playOut(t *testing.T, p Player, seed int64) int {
	t.Helper()
	g := game.New(seed)
	for !g.Over() {
		m, _, err := p.Pick(context.Background(), g.Board)
		if err != nil {
			t.Fatalf("%s: %v", p.Name(), err)
		}
		if err := g.Step(m); err != nil {
			t.Fatalf("%s picked %s: %v", p.Name(), m, err)
		}
	}
	return g.Score
}

func TestClassicPlayersFinishGamesWithLegalMoves(t *testing.T) {
	for _, p := range []Player{NewRandom(1), NewGreedy(), NewExpectimax(1)} {
		if score := playOut(t, p, 3); score <= 0 {
			t.Errorf("%s: score = %d, want > 0", p.Name(), score)
		}
	}
}

func TestClassicPlayersReportNoLegalMoves(t *testing.T) {
	for _, p := range []Player{NewRandom(1), NewGreedy(), NewExpectimax(1)} {
		if _, _, err := p.Pick(context.Background(), stuck); !errors.Is(err, ErrNoLegalMoves) {
			t.Errorf("%s: err = %v, want ErrNoLegalMoves", p.Name(), err)
		}
	}
}

func TestRandomIsDeterministicPerSeed(t *testing.T) {
	if a, b := playOut(t, NewRandom(5), 9), playOut(t, NewRandom(5), 9); a != b {
		t.Fatalf("same seeds gave %d and %d", a, b)
	}
}

func TestGreedyPrefersHigherGain(t *testing.T) {
	b := game.Board{
		{2, 0, 0, 0},
		{4, 0, 0, 0},
		{4, 0, 0, 0},
		{8, 0, 0, 0},
	}
	// up and down both gain 8, right gains 0, left is illegal; ties go to AllMoves order.
	m, _, err := NewGreedy().Pick(context.Background(), b)
	if err != nil || m != game.Up {
		t.Fatalf("got %s, %v; want up", m, err)
	}
}

func TestGreedyBreaksTiesByEmptyCells(t *testing.T) {
	b := game.Board{
		{4, 0, 0, 0},
		{4, 2, 2, 0},
		{0, 0, 0, 0},
		{2, 0, 0, 2},
	}
	// up/down: one 4+4 merge (gain 8, 11 empty). left/right: two 2+2 merges (gain 8, 12 empty).
	m, _, err := NewGreedy().Pick(context.Background(), b)
	if err != nil || m != game.Left {
		t.Fatalf("got %s, %v; want left", m, err)
	}
}

func TestMonotonicity(t *testing.T) {
	ordered := game.Board{
		{64, 32, 16, 8},
		{32, 16, 8, 4},
		{16, 8, 4, 2},
		{8, 4, 2, 0},
	}
	if got := monotonicity(ordered); got != 0 {
		t.Errorf("ordered board: %v, want 0", got)
	}
	if got := monotonicity(game.Board{{2, 8, 2, 8}}); got >= 0 {
		t.Errorf("zigzag row: %v, want negative", got)
	}
}

func TestExpectimaxBeatsRandom(t *testing.T) {
	var ex, rnd int
	for seed := int64(1); seed <= 3; seed++ {
		ex += playOut(t, NewExpectimax(2), seed)
		rnd += playOut(t, NewRandom(seed), seed)
	}
	if ex <= rnd {
		t.Fatalf("expectimax total %d <= random total %d", ex, rnd)
	}
}
