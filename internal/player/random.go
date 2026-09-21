package player

import (
	"context"
	"math/rand"

	"github.com/tatsuo48/jev-poc/internal/game"
)

// Random is not safe for concurrent use; create one per game.
type Random struct{ rng *rand.Rand }

func NewRandom(seed int64) *Random {
	return &Random{rng: rand.New(rand.NewSource(seed))}
}

func (*Random) Name() string { return "random" }

func (p *Random) Pick(_ context.Context, b game.Board) (game.Move, Info, error) {
	legal := game.LegalMoves(b)
	if len(legal) == 0 {
		return 0, Info{}, ErrNoLegalMoves
	}
	return legal[p.rng.Intn(len(legal))], Info{}, nil
}
