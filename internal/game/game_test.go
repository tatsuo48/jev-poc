package game

import (
	"errors"
	"math/rand"
	"testing"
)

func TestNewStartsWithTwoTiles(t *testing.T) {
	g := New(1)
	if got := EmptyCells(g.Board); got != 14 {
		t.Fatalf("EmptyCells = %d, want 14", got)
	}
	if g.Score != 0 || g.Moves != 0 {
		t.Fatalf("score=%d moves=%d, want 0 0", g.Score, g.Moves)
	}
}

func TestSameSeedSameGame(t *testing.T) {
	a, b := New(7), New(7)
	for i := 0; i < 50 && !a.Over(); i++ {
		m := LegalMoves(a.Board)[0]
		if err := a.Step(m); err != nil {
			t.Fatal(err)
		}
		if err := b.Step(m); err != nil {
			t.Fatal(err)
		}
		if a.Board != b.Board || a.Score != b.Score {
			t.Fatalf("diverged at move %d", i)
		}
	}
	if a.Moves == 0 {
		t.Fatal("no moves were played")
	}
}

func TestStepRejectsIllegalMove(t *testing.T) {
	g := New(1)
	g.Board = Board{{2, 4, 2, 4}}
	before := g.Board
	if err := g.Step(Left); !errors.Is(err, ErrIllegalMove) {
		t.Fatalf("err = %v, want ErrIllegalMove", err)
	}
	if g.Board != before || g.Moves != 0 {
		t.Fatal("an illegal move changed the game")
	}
}

func TestStepAddsScoreAndSpawns(t *testing.T) {
	g := New(1)
	g.Board = Board{{2, 2, 0, 0}}
	if err := g.Step(Left); err != nil {
		t.Fatal(err)
	}
	if g.Score != 4 || g.Moves != 1 {
		t.Fatalf("score=%d moves=%d, want 4 1", g.Score, g.Moves)
	}
	if got := EmptyCells(g.Board); got != 14 {
		t.Fatalf("EmptyCells = %d, want 14 (merged tile plus one spawn)", got)
	}
}

func TestOver(t *testing.T) {
	g := New(1)
	if g.Over() {
		t.Fatal("a fresh game is over")
	}
	g.Board = Board{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 2},
	}
	if !g.Over() {
		t.Fatal("a stuck board is not over")
	}
}

func TestSpawn(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	full := Board{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 2},
	}
	if got := Spawn(full, r); got != full {
		t.Fatal("Spawn changed a full board")
	}
	oneHole := full
	oneHole[2][1] = 0
	got := Spawn(oneHole, r)
	if v := got[2][1]; v != 2 && v != 4 {
		t.Fatalf("spawned %d, want 2 or 4", v)
	}
}
