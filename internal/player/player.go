// Package player defines the Player interface and its implementations.
package player

import (
	"context"
	"errors"
	"time"

	"github.com/tatsuo48/jev-poc/internal/game"
)

var ErrNoLegalMoves = errors.New("no legal moves")

// Info carries what a player knows about its own decision.
type Info struct {
	Probabilities map[game.Move]float64 // nil unless the player is jev-backed
	Confidence    float64
	Latency       time.Duration // filled in by the runner
	InputTokens   int
	OutputTokens  int
}

type Player interface {
	Name() string
	Pick(ctx context.Context, b game.Board) (game.Move, Info, error)
}
