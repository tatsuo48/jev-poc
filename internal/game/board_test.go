package game

import (
	"reflect"
	"testing"
)

func TestSlideRowRules(t *testing.T) {
	tests := []struct {
		name   string
		row    [4]int
		want   [4]int
		gained int
	}{
		{"all same merges pairwise", [4]int{2, 2, 2, 2}, [4]int{4, 4, 0, 0}, 8},
		{"merged tile does not merge again", [4]int{2, 2, 4, 0}, [4]int{4, 4, 0, 0}, 4},
		{"merge across a gap", [4]int{4, 0, 4, 8}, [4]int{8, 8, 0, 0}, 8},
		{"three same merges the leading pair", [4]int{2, 2, 2, 0}, [4]int{4, 2, 0, 0}, 4},
		{"nothing to merge", [4]int{2, 4, 2, 4}, [4]int{2, 4, 2, 4}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, gained, _ := Slide(Board{tt.row}, Left)
			if next[0] != tt.want || gained != tt.gained {
				t.Fatalf("got %v gained=%d, want %v gained=%d", next[0], gained, tt.want, tt.gained)
			}
		})
	}
}

func TestSlideFourDirections(t *testing.T) {
	b := Board{
		{2, 0, 0, 2},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{2, 0, 0, 2},
	}
	tests := []struct {
		move Move
		want Board
	}{
		{Left, Board{{4, 0, 0, 0}, {}, {}, {4, 0, 0, 0}}},
		{Right, Board{{0, 0, 0, 4}, {}, {}, {0, 0, 0, 4}}},
		{Up, Board{{4, 0, 0, 4}, {}, {}, {}}},
		{Down, Board{{}, {}, {}, {4, 0, 0, 4}}},
	}
	for _, tt := range tests {
		t.Run(tt.move.String(), func(t *testing.T) {
			next, gained, moved := Slide(b, tt.move)
			if next != tt.want || gained != 8 || !moved {
				t.Fatalf("got %v gained=%d moved=%v, want %v gained=8 moved=true", next, gained, moved, tt.want)
			}
			if got := CountMerges(b, tt.move); got != 2 {
				t.Fatalf("CountMerges = %d, want 2", got)
			}
		})
	}
}

func TestSlideReportsNotMoved(t *testing.T) {
	b := Board{{2, 4, 2, 4}}
	for _, m := range []Move{Left, Right, Up} {
		if next, _, moved := Slide(b, m); moved || next != b {
			t.Errorf("%s: moved=%v next=%v, want unchanged", m, moved, next)
		}
	}
}

func TestLegalMoves(t *testing.T) {
	if got := LegalMoves(Board{{2, 4, 2, 4}}); !reflect.DeepEqual(got, []Move{Down}) {
		t.Errorf("got %v, want [down]", got)
	}
	if got := LegalMoves(Board{{2}}); !reflect.DeepEqual(got, []Move{Down, Right}) {
		t.Errorf("got %v, want [down right]", got)
	}
	stuck := Board{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 2},
	}
	if got := LegalMoves(stuck); len(got) != 0 {
		t.Errorf("stuck board: got %v, want none", got)
	}
}

func TestBoardFeatures(t *testing.T) {
	b := Board{
		{64, 2, 0, 0},
		{0, 0, 0, 0},
		{0, 8, 0, 0},
		{0, 0, 0, 0},
	}
	if got := EmptyCells(b); got != 13 {
		t.Errorf("EmptyCells = %d, want 13", got)
	}
	if got := MaxTile(b); got != 64 {
		t.Errorf("MaxTile = %d, want 64", got)
	}
	if !MaxInCorner(b) {
		t.Error("MaxInCorner = false, want true")
	}
	b[0][0], b[1][1] = 0, 64
	if MaxInCorner(b) {
		t.Error("MaxInCorner = true for a center tile, want false")
	}
	if MaxInCorner(Board{}) {
		t.Error("MaxInCorner = true for an empty board, want false")
	}
}

func TestParseMoveRoundTrip(t *testing.T) {
	for _, m := range AllMoves {
		got, ok := ParseMove(m.String())
		if !ok || got != m {
			t.Errorf("ParseMove(%q) = %v, %v", m.String(), got, ok)
		}
	}
	if _, ok := ParseMove("sideways"); ok {
		t.Error("ParseMove accepted an unknown move")
	}
}
