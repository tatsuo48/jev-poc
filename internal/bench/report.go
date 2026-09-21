package bench

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/tatsuo48/jev-poc/internal/jev"
	"github.com/tatsuo48/jev-poc/internal/runner"
)

// Summary aggregates one player's games. Score statistics cover finished
// games only; latency and tokens cover everything that was spent.
type Summary struct {
	Player     string
	Games      int // finished games
	Errors     int // aborted games
	AvgScore   float64
	BestScore  int
	MaxTiles   map[int]int // max tile -> number of finished games
	AvgMoves   float64
	AvgLatency time.Duration // per move
	Tokens     int
}

func Summarize(players []string, results []runner.Result) []Summary {
	summaries := make([]Summary, 0, len(players))
	for _, name := range players {
		s := Summary{Player: name, MaxTiles: map[int]int{}}
		var score, moves, allMoves int
		var latency time.Duration
		for _, r := range results {
			if r.Player != name {
				continue
			}
			s.Tokens += r.InputTokens + r.OutputTokens
			latency += r.Latency
			allMoves += r.Moves
			if r.Err != nil {
				s.Errors++
				continue
			}
			s.Games++
			score += r.Score
			moves += r.Moves
			s.MaxTiles[r.MaxTile]++
			if r.Score > s.BestScore {
				s.BestScore = r.Score
			}
		}
		if s.Games > 0 {
			s.AvgScore = float64(score) / float64(s.Games)
			s.AvgMoves = float64(moves) / float64(s.Games)
		}
		if allMoves > 0 {
			s.AvgLatency = latency / time.Duration(allMoves)
		}
		summaries = append(summaries, s)
	}
	return summaries
}

func WriteTable(w io.Writer, summaries []Summary) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PLAYER\tGAMES\tERRORS\tAVG SCORE\tBEST\tMAX TILES\tAVG MOVES\tLATENCY/MOVE\tTOKENS")
	for _, s := range summaries {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%.0f\t%d\t%s\t%.0f\t%s\t%d\n",
			s.Player, s.Games, s.Errors, s.AvgScore, s.BestScore, formatTiles(s.MaxTiles), s.AvgMoves, formatLatency(s.AvgLatency), s.Tokens)
	}
	tw.Flush()
}

func formatTiles(tiles map[int]int) string {
	if len(tiles) == 0 {
		return "-"
	}
	keys := make([]int, 0, len(tiles))
	for k := range tiles {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(keys)))
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%dx%d", k, tiles[k])
	}
	return strings.Join(parts, " ")
}

func formatLatency(d time.Duration) string {
	if d < time.Millisecond {
		return d.Round(time.Microsecond).String()
	}
	return d.Round(time.Millisecond).String()
}

// BudgetExceeded reports whether any game stopped because the jev call budget ran out.
func BudgetExceeded(results []runner.Result) bool {
	for _, r := range results {
		if errors.Is(r.Err, jev.ErrBudgetExceeded) {
			return true
		}
	}
	return false
}
