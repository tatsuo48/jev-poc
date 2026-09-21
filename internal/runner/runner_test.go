package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/tatsuo48/jev-poc/internal/game"
	"github.com/tatsuo48/jev-poc/internal/player"
)

type firstLegal struct{}

func (firstLegal) Name() string { return "first-legal" }
func (firstLegal) Pick(_ context.Context, b game.Board) (game.Move, player.Info, error) {
	return game.LegalMoves(b)[0], player.Info{InputTokens: 10, OutputTokens: 1}, nil
}

type failing struct{ err error }

func (failing) Name() string { return "failing" }
func (f failing) Pick(context.Context, game.Board) (game.Move, player.Info, error) {
	return 0, player.Info{}, f.err
}

type illegal struct{}

func (illegal) Name() string { return "illegal" }
func (illegal) Pick(_ context.Context, b game.Board) (game.Move, player.Info, error) {
	for _, m := range game.AllMoves {
		if _, _, moved := game.Slide(b, m); !moved {
			return m, player.Info{}, nil
		}
	}
	return game.LegalMoves(b)[0], player.Info{}, nil
}

func TestPlayGameIsDeterministic(t *testing.T) {
	a := PlayGame(context.Background(), firstLegal{}, 4, nil)
	b := PlayGame(context.Background(), firstLegal{}, 4, nil)
	if a.Err != nil || a.Moves == 0 {
		t.Fatalf("unexpected result: %+v", a)
	}
	if a.Score != b.Score || a.Moves != b.Moves || a.MaxTile != b.MaxTile {
		t.Fatalf("same seed differed: %+v vs %+v", a, b)
	}
	if a.Player != "first-legal" || a.Seed != 4 {
		t.Fatalf("labels: %+v", a)
	}
	if a.InputTokens != 10*a.Moves || a.OutputTokens != a.Moves {
		t.Fatalf("tokens not accumulated: %+v", a)
	}
}

func TestPlayGameNotifiesObserverPerMove(t *testing.T) {
	var steps []Step
	res := PlayGame(context.Background(), firstLegal{}, 4, func(s Step) { steps = append(steps, s) })
	if len(steps) != res.Moves {
		t.Fatalf("observed %d steps, played %d moves", len(steps), res.Moves)
	}
	first, last := steps[0], steps[len(steps)-1]
	if first.MoveNo != 1 || first.Before == first.After {
		t.Fatalf("first step: %+v", first)
	}
	if last.Score != res.Score || last.Seed != 4 {
		t.Fatalf("last step: %+v, result: %+v", last, res)
	}
}

func TestPlayGameStopsOnPlayerError(t *testing.T) {
	boom := errors.New("boom")
	res := PlayGame(context.Background(), failing{boom}, 1, nil)
	if !errors.Is(res.Err, boom) || res.Moves != 0 {
		t.Fatalf("got %+v", res)
	}
}

func TestPlayGameStopsOnIllegalMove(t *testing.T) {
	res := PlayGame(context.Background(), illegal{}, 1, nil)
	if !errors.Is(res.Err, game.ErrIllegalMove) {
		t.Fatalf("err = %v, want ErrIllegalMove", res.Err)
	}
}

func TestPlayGameStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := PlayGame(ctx, firstLegal{}, 1, nil)
	if !errors.Is(res.Err, context.Canceled) || res.Moves != 0 {
		t.Fatalf("got %+v", res)
	}
}
