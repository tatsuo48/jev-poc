package player

import (
	"context"
	"math"

	"github.com/tatsuo48/jev-poc/internal/game"
)

// Expectimax looks depth player-moves ahead, averaging over tile spawns.
type Expectimax struct{ depth int }

func NewExpectimax(depth int) *Expectimax {
	if depth < 1 {
		depth = 1
	}
	return &Expectimax{depth: depth}
}

func (*Expectimax) Name() string { return "expectimax" }

func (e *Expectimax) Pick(_ context.Context, b game.Board) (game.Move, Info, error) {
	legal := game.LegalMoves(b)
	if len(legal) == 0 {
		return 0, Info{}, ErrNoLegalMoves
	}
	best, bestValue := legal[0], math.Inf(-1)
	for _, m := range legal {
		next, _, _ := game.Slide(b, m)
		if v := e.chance(next, e.depth-1); v > bestValue {
			best, bestValue = m, v
		}
	}
	return best, Info{}, nil
}

// chance is the expected value over every possible spawn on b.
func (e *Expectimax) chance(b game.Board, depth int) float64 {
	if depth == 0 {
		return heuristic(b)
	}
	sum, cells := 0.0, 0
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if b[r][c] != 0 {
				continue
			}
			cells++
			with2, with4 := b, b
			with2[r][c], with4[r][c] = 2, 4
			sum += 0.9*e.best(with2, depth) + 0.1*e.best(with4, depth)
		}
	}
	if cells == 0 {
		return e.best(b, depth)
	}
	return sum / float64(cells)
}

// best is the value of the player's best move on b.
func (e *Expectimax) best(b game.Board, depth int) float64 {
	legal := game.LegalMoves(b)
	if len(legal) == 0 {
		return -1e6
	}
	value := math.Inf(-1)
	for _, m := range legal {
		next, _, _ := game.Slide(b, m)
		value = math.Max(value, e.chance(next, depth-1))
	}
	return value
}

func heuristic(b game.Board) float64 {
	h := 30*float64(game.EmptyCells(b)) + 10*monotonicity(b)
	if game.MaxInCorner(b) {
		h += 20 * log2(game.MaxTile(b))
	}
	return h
}

func log2(v int) float64 {
	if v == 0 {
		return 0
	}
	return math.Log2(float64(v))
}

// monotonicity is 0 when every row and column is sorted in one direction and
// grows more negative the more the lines zigzag.
func monotonicity(b game.Board) float64 {
	total := 0.0
	for i := 0; i < 4; i++ {
		var rowInc, rowDec, colInc, colDec float64
		for k := 0; k < 3; k++ {
			if d := log2(b[i][k+1]) - log2(b[i][k]); d > 0 {
				rowInc += d
			} else {
				rowDec -= d
			}
			if d := log2(b[k+1][i]) - log2(b[k][i]); d > 0 {
				colInc += d
			} else {
				colDec -= d
			}
		}
		total -= math.Min(rowInc, rowDec) + math.Min(colInc, colDec)
	}
	return total
}
