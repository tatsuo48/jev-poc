package player

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/tatsuo48/jev-poc/internal/game"
	"github.com/tatsuo48/jev-poc/internal/jev"
)

type fakeAsker struct {
	calls     int
	state     any
	questions map[string]jev.Question
	resp      jev.Response
	err       error
}

func (f *fakeAsker) Ask(_ context.Context, state any, questions map[string]jev.Question) (jev.Response, error) {
	f.calls++
	f.state, f.questions = state, questions
	return f.resp, f.err
}

func answer(choice string, probs map[string]float64) jev.Response {
	return jev.Response{
		Answers: map[string]jev.Answer{"move": {Type: "choice", Choice: choice, Probabilities: probs, Confidence: 0.4}},
		Usage:   jev.Usage{InputTokens: 120, OutputTokens: 8},
	}
}

// Only down and right are legal on this board.
var cornerTile = game.Board{{2}}

func stateJSON(t *testing.T, state any) map[string]any {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestJevPlayersOfferOnlyLegalMoves(t *testing.T) {
	for _, newPlayer := range []func(Asker) Player{NewJevRaw, NewJevSim} {
		f := &fakeAsker{resp: answer("right", map[string]float64{"down": 0.3, "right": 0.7})}
		p := newPlayer(f)

		m, info, err := p.Pick(context.Background(), cornerTile)
		if err != nil || m != game.Right {
			t.Fatalf("%s: got %s, %v; want right", p.Name(), m, err)
		}
		q := f.questions["move"]
		var options []string
		for k := range q.Criteria {
			options = append(options, k)
		}
		sort.Strings(options)
		if q.Type != "choice" || !reflect.DeepEqual(options, []string{"down", "right"}) {
			t.Errorf("%s: question = %+v", p.Name(), q)
		}
		want := map[game.Move]float64{game.Down: 0.3, game.Right: 0.7}
		if !reflect.DeepEqual(info.Probabilities, want) || info.Confidence != 0.4 {
			t.Errorf("%s: info = %+v", p.Name(), info)
		}
		if info.InputTokens != 120 || info.OutputTokens != 8 {
			t.Errorf("%s: tokens = %+v", p.Name(), info)
		}
	}
}

func TestJevPlayersSkipAPIWhenOnlyOneMoveIsLegal(t *testing.T) {
	for _, newPlayer := range []func(Asker) Player{NewJevRaw, NewJevSim} {
		f := &fakeAsker{}
		p := newPlayer(f)
		m, info, err := p.Pick(context.Background(), game.Board{{2, 4, 2, 4}})
		if err != nil || m != game.Down || f.calls != 0 {
			t.Fatalf("%s: got %s, %v, calls=%d", p.Name(), m, err, f.calls)
		}
		if info.Probabilities != nil {
			t.Errorf("%s: probabilities without an API call", p.Name())
		}
	}
}

func TestJevPlayersRejectBadAnswers(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name  string
		asker *fakeAsker
	}{
		{"illegal choice", &fakeAsker{resp: answer("up", nil)}},
		{"unknown choice", &fakeAsker{resp: answer("sideways", nil)}},
		{"missing answer", &fakeAsker{resp: jev.Response{}}},
		{"api error", &fakeAsker{err: boom}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := NewJevRaw(tt.asker).Pick(context.Background(), cornerTile); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
	if _, _, err := NewJevRaw(&fakeAsker{err: boom}).Pick(context.Background(), cornerTile); !errors.Is(err, boom) {
		t.Fatalf("api error was not wrapped: %v", err)
	}
	if _, _, err := NewJevRaw(&fakeAsker{}).Pick(context.Background(), stuck); !errors.Is(err, ErrNoLegalMoves) {
		t.Fatalf("err = %v, want ErrNoLegalMoves", err)
	}
}

func TestJevRawStateIsTheBoardOnly(t *testing.T) {
	f := &fakeAsker{resp: answer("down", nil)}
	if _, _, err := NewJevRaw(f).Pick(context.Background(), cornerTile); err != nil {
		t.Fatal(err)
	}
	state := stateJSON(t, f.state)
	if _, ok := state["board"]; !ok || len(state) != 1 {
		t.Fatalf("state = %v", state)
	}
}

func TestJevSimStateDescribesEachCandidate(t *testing.T) {
	b := game.Board{
		{2, 2, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}
	f := &fakeAsker{resp: answer("left", nil)}
	if _, _, err := NewJevSim(f).Pick(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	state := stateJSON(t, f.state)
	candidates := state["candidates"].(map[string]any)
	if len(candidates) != 3 { // up is illegal
		t.Fatalf("candidates = %v", candidates)
	}
	left := candidates["left"].(map[string]any)
	want := map[string]any{"gained": 4.0, "empty_cells": 15.0, "merges": 1.0, "max_tile": 4.0, "max_in_corner": true}
	for k, v := range want {
		if left[k] != v {
			t.Errorf("left.%s = %v, want %v", k, left[k], v)
		}
	}
	if _, ok := left["board"]; !ok {
		t.Error("left.board is missing")
	}
	down := candidates["down"].(map[string]any)
	if down["gained"] != 0.0 || down["merges"] != 0.0 || down["empty_cells"] != 14.0 {
		t.Errorf("down = %v", down)
	}
}
