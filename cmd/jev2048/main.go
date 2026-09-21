// Command jev2048 lets TypeSafe AI's jev model play 2048 against classic AIs.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/tatsuo48/jev-poc/internal/bench"
	"github.com/tatsuo48/jev-poc/internal/jev"
	"github.com/tatsuo48/jev-poc/internal/player"
	"github.com/tatsuo48/jev-poc/internal/watch"
)

const usage = `usage:
  jev2048 watch [--player NAME] [--seed N] [--delay D] [--max-calls N]
  jev2048 bench [--players A,B,...] [--games N] [--seed N] [--parallel N] [--max-calls N] [--out FILE]

players: %s
jev players read the API key from TYPESAFE_API_KEY.
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, usage, strings.Join(player.Names, ", "))
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch args[0] {
	case "watch":
		return runWatch(ctx, args[1:])
	case "bench":
		return runBench(ctx, args[1:])
	}
	fmt.Fprintf(os.Stderr, usage, strings.Join(player.Names, ", "))
	return 2
}

// newClient returns nil when none of the players talks to jev, so that the
// classic players work without an API key.
func newClient(names []string, maxCalls int) (*jev.Client, error) {
	for _, name := range names {
		if player.NeedsJev(name) {
			return jev.NewFromEnv(maxCalls)
		}
	}
	return nil, nil
}

// asker avoids wrapping a nil *jev.Client in a non-nil interface.
func asker(c *jev.Client) player.Asker {
	if c == nil {
		return nil
	}
	return c
}

func runWatch(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	name := fs.String("player", "jev-sim", "player to watch")
	seed := fs.Int64("seed", 42, "game seed")
	delay := fs.Duration("delay", 100*time.Millisecond, "pause between moves")
	maxCalls := fs.Int("max-calls", 5000, "upper bound on jev API calls (0 = unlimited)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	client, err := newClient([]string{*name}, *maxCalls)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	p, err := player.New(*name, *seed, asker(client))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if res := watch.Run(ctx, os.Stdout, p, *seed, *delay); res.Err != nil {
		return 1
	}
	return 0
}

func runBench(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	players := fs.String("players", "random,greedy,expectimax", "comma-separated players")
	games := fs.Int("games", 5, "games per player")
	seed := fs.Int64("seed", 1, "first seed; game i uses seed+i")
	parallel := fs.Int("parallel", 4, "games played at the same time")
	maxCalls := fs.Int("max-calls", 20000, "upper bound on jev API calls (0 = unlimited)")
	outPath := fs.String("out", "", "write one JSON line per move to this file")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	names := strings.Split(*players, ",")
	for _, name := range names {
		if _, err := player.New(name, 0, noopAsker{}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	client, err := newClient(names, *maxCalls)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	var out io.Writer
	if *outPath != "" {
		f, err := os.Create(*outPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer f.Close()
		out = f
	}

	results := bench.Run(ctx, bench.Config{
		Players:  names,
		Games:    *games,
		Seed:     *seed,
		Parallel: *parallel,
		Out:      out,
		New: func(name string, seed int64) (player.Player, error) {
			return player.New(name, seed, asker(client))
		},
	})

	bench.WriteTable(os.Stdout, bench.Summarize(names, results))
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(os.Stderr, "aborted: player=%s seed=%d after %d moves: %v\n", r.Player, r.Seed, r.Moves, r.Err)
		}
	}
	if client != nil {
		s := client.Stats()
		fmt.Printf("\njev API: calls=%d input_tokens=%d output_tokens=%d\n", s.Calls, s.InputTokens, s.OutputTokens)
	}
	if bench.BudgetExceeded(results) {
		fmt.Fprintln(os.Stderr, "stopped: --max-calls was reached")
		return 1
	}
	if ctx.Err() != nil {
		return 1
	}
	return 0
}

// noopAsker lets runBench validate player names before it needs an API key.
type noopAsker struct{}

func (noopAsker) Ask(context.Context, any, map[string]jev.Question) (jev.Response, error) {
	return jev.Response{}, nil
}
