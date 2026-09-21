// Package runner plays one game to the end with a given player.
package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/tatsuo48/jev-poc/internal/game"
	"github.com/tatsuo48/jev-poc/internal/player"
)

// Step describes one played move.
type Step struct {
	Seed   int64
	MoveNo int // 1-based
	Before game.Board
	After  game.Board
	Score  int // score after this move
	Move   game.Move
	Info   player.Info
}

type Observer func(Step)

// Result reports the outcome of one played game. When Err is non-nil, the
// game was aborted before it finished: Score, Moves, and MaxTile describe
// only the partial game up to the point of the abort, and callers must not
// average them together with finished games (see bench.Summarize, which
// excludes them from its score/move averages but still counts their
// Latency and tokens).
type Result struct {
	Player       string
	Seed         int64
	Score        int
	Moves        int
	MaxTile      int
	Latency      time.Duration // total time spent inside Pick
	InputTokens  int
	OutputTokens int
	Err          error // non-nil when the game was aborted
}

// PlayGame never substitutes a move for a failing player: the first error
// aborts the game and is reported in Result.Err.
func PlayGame(ctx context.Context, p player.Player, seed int64, obs Observer) Result {
	g := game.New(seed)
	res := Result{Player: p.Name(), Seed: seed}
	for !g.Over() {
		if err := ctx.Err(); err != nil {
			res.Err = err
			break
		}
		before := g.Board
		start := time.Now()
		m, info, err := p.Pick(ctx, before)
		info.Latency = time.Since(start)
		res.Latency += info.Latency
		res.InputTokens += info.InputTokens
		res.OutputTokens += info.OutputTokens
		if err != nil {
			res.Err = err
			break
		}
		if err := g.Step(m); err != nil {
			res.Err = fmt.Errorf("%s picked %s: %w", p.Name(), m, err)
			break
		}
		if obs != nil {
			obs(Step{Seed: seed, MoveNo: g.Moves, Before: before, After: g.Board, Score: g.Score, Move: m, Info: info})
		}
	}
	res.Score, res.Moves, res.MaxTile = g.Score, g.Moves, game.MaxTile(g.Board)
	return res
}
