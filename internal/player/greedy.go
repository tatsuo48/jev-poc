package player

import (
	"context"

	"github.com/tatsuo48/jev-poc/internal/game"
)

// Greedy maximizes the immediate score gain, then the number of empty cells.
type Greedy struct{}

func NewGreedy() *Greedy { return &Greedy{} }

func (*Greedy) Name() string { return "greedy" }

func (*Greedy) Pick(_ context.Context, b game.Board) (game.Move, Info, error) {
	legal := game.LegalMoves(b)
	if len(legal) == 0 {
		return 0, Info{}, ErrNoLegalMoves
	}
	best, bestGain, bestEmpty := legal[0], -1, -1
	for _, m := range legal {
		next, gained, _ := game.Slide(b, m)
		empty := game.EmptyCells(next)
		if gained > bestGain || (gained == bestGain && empty > bestEmpty) {
			best, bestGain, bestEmpty = m, gained, empty
		}
	}
	return best, Info{}, nil
}
