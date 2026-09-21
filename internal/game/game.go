package game

import (
	"errors"
	"math/rand"
)

var ErrIllegalMove = errors.New("illegal move")

// Game is one playthrough. The same seed and the same moves always produce
// the same sequence of boards.
type Game struct {
	Board Board
	Score int
	Moves int
	rng   *rand.Rand
}

func New(seed int64) *Game {
	g := &Game{rng: rand.New(rand.NewSource(seed))}
	g.Board = Spawn(g.Board, g.rng)
	g.Board = Spawn(g.Board, g.rng)
	return g
}

// Spawn puts a 2 (90%) or a 4 (10%) on a random empty cell.
func Spawn(b Board, r *rand.Rand) Board {
	var empty [][2]int
	for i, row := range b {
		for j, v := range row {
			if v == 0 {
				empty = append(empty, [2]int{i, j})
			}
		}
	}
	if len(empty) == 0 {
		return b
	}
	p := empty[r.Intn(len(empty))]
	v := 2
	if r.Intn(10) == 0 {
		v = 4
	}
	b[p[0]][p[1]] = v
	return b
}

func (g *Game) Step(m Move) error {
	next, gained, moved := Slide(g.Board, m)
	if !moved {
		return ErrIllegalMove
	}
	g.Board = Spawn(next, g.rng)
	g.Score += gained
	g.Moves++
	return nil
}

func (g *Game) Over() bool {
	return len(LegalMoves(g.Board)) == 0
}
