// Package watch renders a game move by move in the terminal.
package watch

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/bits"
	"strings"
	"time"

	"github.com/tatsuo48/jev-poc/internal/game"
	"github.com/tatsuo48/jev-poc/internal/player"
	"github.com/tatsuo48/jev-poc/internal/runner"
)

const (
	clearScreen = "\x1b[H\x1b[2J"
	barWidth    = 20
)

// tileColors maps log2(tile) to a 256-color background; index 0 is unused.
var tileColors = []int{0, 252, 230, 215, 209, 203, 196, 228, 227, 226, 220, 214, 93}

type Totals struct {
	InputTokens  int
	OutputTokens int
}

func Render(w io.Writer, playerName string, s runner.Step, tot Totals) {
	var sb strings.Builder
	sb.WriteString(clearScreen)
	fmt.Fprintf(&sb, "jev-2048  player=%s  seed=%d\n", playerName, s.Seed)
	fmt.Fprintf(&sb, "score=%d  moves=%d  last=%s  latency=%s", s.Score, s.MoveNo, s.Move, s.Info.Latency.Round(time.Millisecond))
	if tokens := tot.InputTokens + tot.OutputTokens; tokens > 0 {
		fmt.Fprintf(&sb, "  tokens=%d", tokens)
	}
	sb.WriteString("\n\n")

	for _, row := range s.After {
		for _, v := range row {
			sb.WriteString(tile(v))
		}
		sb.WriteString("\n")
	}

	if s.Info.Probabilities != nil {
		sb.WriteString("\n")
		for _, m := range game.AllMoves {
			p, ok := s.Info.Probabilities[m]
			if !ok {
				continue
			}
			filled := min(max(int(math.Round(p*barWidth)), 0), barWidth)
			fmt.Fprintf(&sb, "%-5s %s%s %.2f\n", m, strings.Repeat("█", filled), strings.Repeat("░", barWidth-filled), p)
		}
		fmt.Fprintf(&sb, "confidence=%.2f\n", s.Info.Confidence)
	}
	io.WriteString(w, sb.String())
}

func tile(v int) string {
	if v == 0 {
		return "\x1b[48;5;236m      \x1b[0m"
	}
	idx := bits.Len(uint(v)) - 1
	if idx >= len(tileColors) {
		idx = len(tileColors) - 1
	}
	return fmt.Sprintf("\x1b[48;5;%dm\x1b[38;5;16m%5d \x1b[0m", tileColors[idx], v)
}

// Run plays one game, redrawing after every move and pausing delay between moves.
func Run(ctx context.Context, w io.Writer, p player.Player, seed int64, delay time.Duration) runner.Result {
	var tot Totals
	res := runner.PlayGame(ctx, p, seed, func(s runner.Step) {
		tot.InputTokens += s.Info.InputTokens
		tot.OutputTokens += s.Info.OutputTokens
		Render(w, p.Name(), s, tot)
		if delay > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(delay):
			}
		}
	})
	if res.Err != nil {
		fmt.Fprintf(w, "\nerror: %v\n", res.Err)
	} else {
		fmt.Fprintf(w, "\nGame over. score=%d moves=%d max_tile=%d\n", res.Score, res.Moves, res.MaxTile)
	}
	return res
}
