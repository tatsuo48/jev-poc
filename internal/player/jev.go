package player

import (
	"context"
	"fmt"

	"github.com/tatsuo48/jev-poc/internal/game"
	"github.com/tatsuo48/jev-poc/internal/jev"
)

// Asker is the part of jev.Client the players need.
type Asker interface {
	Ask(ctx context.Context, state any, questions map[string]jev.Question) (jev.Response, error)
}

const (
	questionKey  = "move"
	instructions = "You are playing the game 2048 on a 4x4 board, given as rows from top to bottom (0 = empty cell). " +
		"Choose the move most likely to lead to the highest final score in the long run. " +
		"Good play keeps the largest tile in a corner, keeps many cells empty, " +
		"and keeps rows and columns sorted in one direction."
)

var directionHelp = map[game.Move]string{
	game.Up:    "Slide all tiles toward the top row.",
	game.Down:  "Slide all tiles toward the bottom row.",
	game.Left:  "Slide all tiles toward the leftmost column.",
	game.Right: "Slide all tiles toward the rightmost column.",
}

// promptBuilder turns a board and its legal moves into the state and the
// option descriptions sent to jev.
type promptBuilder func(b game.Board, legal []game.Move) (state any, criteria map[string]string)

type jevPlayer struct {
	name  string
	asker Asker
	build promptBuilder
}

// NewJevRaw shows jev nothing but the board.
func NewJevRaw(a Asker) Player { return &jevPlayer{name: "jev-raw", asker: a, build: rawPrompt} }

// NewJevSim shows jev the outcome of every legal move.
func NewJevSim(a Asker) Player { return &jevPlayer{name: "jev-sim", asker: a, build: simPrompt} }

func (p *jevPlayer) Name() string { return p.name }

func (p *jevPlayer) Pick(ctx context.Context, b game.Board) (game.Move, Info, error) {
	legal := game.LegalMoves(b)
	switch len(legal) {
	case 0:
		return 0, Info{}, ErrNoLegalMoves
	case 1:
		return legal[0], Info{}, nil
	}

	state, criteria := p.build(b, legal)
	resp, err := p.asker.Ask(ctx, state, map[string]jev.Question{
		questionKey: {Type: "choice", Instructions: instructions, Criteria: criteria},
	})
	if err != nil {
		return 0, Info{}, fmt.Errorf("%s: %w", p.name, err)
	}
	info := Info{InputTokens: resp.Usage.InputTokens, OutputTokens: resp.Usage.OutputTokens}

	ans, ok := resp.Answers[questionKey]
	if !ok {
		return 0, info, fmt.Errorf("%s: response has no answer for %q", p.name, questionKey)
	}
	move, ok := game.ParseMove(ans.Choice)
	if !ok || !contains(legal, move) {
		return 0, info, fmt.Errorf("%s: jev chose %q, which is not a legal move", p.name, ans.Choice)
	}

	info.Confidence = ans.Confidence
	info.Probabilities = make(map[game.Move]float64, len(ans.Probabilities))
	for name, prob := range ans.Probabilities {
		if m, ok := game.ParseMove(name); ok {
			info.Probabilities[m] = prob
		}
	}
	return move, info, nil
}

func contains(moves []game.Move, m game.Move) bool {
	for _, x := range moves {
		if x == m {
			return true
		}
	}
	return false
}

func rawPrompt(b game.Board, legal []game.Move) (any, map[string]string) {
	criteria := make(map[string]string, len(legal))
	for _, m := range legal {
		criteria[m.String()] = directionHelp[m]
	}
	return map[string]any{"board": b}, criteria
}

type candidate struct {
	Board       game.Board `json:"board"`
	Gained      int        `json:"gained"`
	EmptyCells  int        `json:"empty_cells"`
	Merges      int        `json:"merges"`
	MaxTile     int        `json:"max_tile"`
	MaxInCorner bool       `json:"max_in_corner"`
}

func simPrompt(b game.Board, legal []game.Move) (any, map[string]string) {
	candidates := make(map[string]candidate, len(legal))
	criteria := make(map[string]string, len(legal))
	for _, m := range legal {
		next, gained, _ := game.Slide(b, m)
		candidates[m.String()] = candidate{
			Board:       next,
			Gained:      gained,
			EmptyCells:  game.EmptyCells(next),
			Merges:      game.CountMerges(b, m),
			MaxTile:     game.MaxTile(next),
			MaxInCorner: game.MaxInCorner(next),
		}
		criteria[m.String()] = fmt.Sprintf("%s The resulting board and its features are in state.candidates.%s.", directionHelp[m], m)
	}
	return map[string]any{"board": b, "candidates": candidates}, criteria
}
