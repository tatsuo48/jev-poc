// Package bench runs players against the same seeds and reports the results.
package bench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/tatsuo48/jev-poc/internal/game"
	"github.com/tatsuo48/jev-poc/internal/jev"
	"github.com/tatsuo48/jev-poc/internal/player"
	"github.com/tatsuo48/jev-poc/internal/runner"
)

type NewPlayer func(name string, seed int64) (player.Player, error)

type Config struct {
	Players  []string
	Games    int
	Seed     int64 // games use Seed, Seed+1, ..., Seed+Games-1
	Parallel int
	New      NewPlayer
	Out      io.Writer // optional JSONL log, one line per move
}

type moveLog struct {
	Player        string             `json:"player"`
	Seed          int64              `json:"seed"`
	MoveNo        int                `json:"move_no"`
	Board         game.Board         `json:"board"` // before the move
	Move          string             `json:"move"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
	LatencyMS     float64            `json:"latency_ms"`
}

// Run plays Games games per player, Parallel games at a time. Hitting the jev
// call budget cancels every game still in flight. Returns the first JSONL write error
// if cfg.Out logging fails, wrapped as "bench: write move log: <err>"; games continue
// to completion.
func Run(ctx context.Context, cfg Config) ([]runner.Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type job struct {
		name string
		seed int64
	}
	jobs := make(chan job)
	var (
		mu      sync.Mutex // guards results, the JSONL encoder, and logErr
		results []runner.Result
		wg      sync.WaitGroup
		enc     *json.Encoder
		logErr  error
	)
	if cfg.Out != nil {
		enc = json.NewEncoder(cfg.Out)
	}

	parallel := cfg.Parallel
	if parallel < 1 {
		parallel = 1
	}
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				res := play(ctx, cfg.New, j.name, j.seed, enc, &mu, &logErr)
				if errors.Is(res.Err, jev.ErrBudgetExceeded) {
					cancel()
				}
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
			}
		}()
	}

	// Feed seed-first (outer loop over games, inner loop over players) so
	// that when the call budget runs out mid-run, every player has played
	// a similar number of games instead of the earliest players getting
	// all their games in while later ones get none.
feed:
	for g := 0; g < cfg.Games; g++ {
		for _, name := range cfg.Players {
			select {
			case <-ctx.Done():
				break feed
			case jobs <- job{name, cfg.Seed + int64(g)}:
			}
		}
	}
	close(jobs)
	wg.Wait()

	order := make(map[string]int, len(cfg.Players))
	for i, name := range cfg.Players {
		order[name] = i
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Player != results[j].Player {
			return order[results[i].Player] < order[results[j].Player]
		}
		return results[i].Seed < results[j].Seed
	})

	var retErr error
	if logErr != nil {
		retErr = fmt.Errorf("bench: write move log: %w", logErr)
	}
	return results, retErr
}

func play(ctx context.Context, newPlayer NewPlayer, name string, seed int64, enc *json.Encoder, mu *sync.Mutex, logErr *error) runner.Result {
	p, err := newPlayer(name, seed)
	if err != nil {
		return runner.Result{Player: name, Seed: seed, Err: err}
	}
	var obs runner.Observer
	if enc != nil {
		obs = func(s runner.Step) {
			probs := make(map[string]float64, len(s.Info.Probabilities))
			for m, v := range s.Info.Probabilities {
				probs[m.String()] = v
			}
			mu.Lock()
			defer mu.Unlock()
			// Only write if we haven't already encountered a write error
			if *logErr == nil {
				if err := enc.Encode(moveLog{
					Player:        name,
					Seed:          seed,
					MoveNo:        s.MoveNo,
					Board:         s.Before,
					Move:          s.Move.String(),
					Probabilities: probs,
					Confidence:    s.Info.Confidence,
					LatencyMS:     float64(s.Info.Latency.Microseconds()) / 1000,
				}); err != nil {
					*logErr = err
				}
			}
		}
	}
	return runner.PlayGame(ctx, p, seed, obs)
}
