// Package game implements the rules of 2048 as pure functions over a 4x4 board.
package game

// Board holds tile values; 0 is an empty cell.
type Board [4][4]int

type Move int

const (
	Up Move = iota
	Down
	Left
	Right
)

// AllMoves fixes the order used for iteration and tie-breaking.
var AllMoves = []Move{Up, Down, Left, Right}

var moveNames = map[Move]string{Up: "up", Down: "down", Left: "left", Right: "right"}

func (m Move) String() string {
	if s, ok := moveNames[m]; ok {
		return s
	}
	return "unknown"
}

func ParseMove(s string) (Move, bool) {
	for m, name := range moveNames {
		if name == s {
			return m, true
		}
	}
	return 0, false
}

// cell returns the coordinates of the k-th cell of line i, counted from the
// edge the tiles travel toward.
func cell(m Move, i, k int) (r, c int) {
	switch m {
	case Left:
		return i, k
	case Right:
		return i, 3 - k
	case Up:
		return k, i
	default:
		return 3 - k, i
	}
}

// slideLine packs a line toward index 0. A tile created by a merge never
// merges again in the same move.
func slideLine(line [4]int) (out [4]int, gained, merges int) {
	idx := 0
	last := 0 // value of the last placed tile that is still mergeable
	for _, v := range line {
		if v == 0 {
			continue
		}
		if v == last {
			out[idx-1] = v * 2
			gained += v * 2
			merges++
			last = 0
			continue
		}
		out[idx] = v
		idx++
		last = v
	}
	return out, gained, merges
}

func slide(b Board, m Move) (next Board, gained, merges int) {
	for i := 0; i < 4; i++ {
		var line [4]int
		for k := 0; k < 4; k++ {
			r, c := cell(m, i, k)
			line[k] = b[r][c]
		}
		out, g, n := slideLine(line)
		gained += g
		merges += n
		for k := 0; k < 4; k++ {
			r, c := cell(m, i, k)
			next[r][c] = out[k]
		}
	}
	return next, gained, merges
}

// Slide applies a move without spawning a new tile.
func Slide(b Board, m Move) (next Board, gained int, moved bool) {
	next, gained, _ = slide(b, m)
	return next, gained, next != b
}

// CountMerges returns how many pairs of tiles the move would merge.
func CountMerges(b Board, m Move) int {
	_, _, merges := slide(b, m)
	return merges
}

// LegalMoves returns the moves that change the board, in AllMoves order.
func LegalMoves(b Board) []Move {
	var legal []Move
	for _, m := range AllMoves {
		if _, _, moved := Slide(b, m); moved {
			legal = append(legal, m)
		}
	}
	return legal
}

func EmptyCells(b Board) int {
	n := 0
	for _, row := range b {
		for _, v := range row {
			if v == 0 {
				n++
			}
		}
	}
	return n
}

func MaxTile(b Board) int {
	max := 0
	for _, row := range b {
		for _, v := range row {
			if v > max {
				max = v
			}
		}
	}
	return max
}

// MaxInCorner reports whether the largest tile sits in one of the four corners.
func MaxInCorner(b Board) bool {
	max := MaxTile(b)
	if max == 0 {
		return false
	}
	return b[0][0] == max || b[0][3] == max || b[3][0] == max || b[3][3] == max
}
