package player

import (
	"fmt"
	"strings"
)

const expectimaxDepth = 3

// Names lists every player New can build.
var Names = []string{"random", "greedy", "expectimax", "jev-raw", "jev-sim"}

func NeedsJev(name string) bool { return strings.HasPrefix(name, "jev-") }

// New builds a fresh player. Players may hold per-game state, so callers
// create one per game. asker may be nil unless NeedsJev(name).
func New(name string, seed int64, asker Asker) (Player, error) {
	if NeedsJev(name) && asker == nil {
		return nil, fmt.Errorf("player %q needs a jev client", name)
	}
	switch name {
	case "random":
		return NewRandom(seed), nil
	case "greedy":
		return NewGreedy(), nil
	case "expectimax":
		return NewExpectimax(expectimaxDepth), nil
	case "jev-raw":
		return NewJevRaw(asker), nil
	case "jev-sim":
		return NewJevSim(asker), nil
	}
	return nil, fmt.Errorf("unknown player %q (available: %s)", name, strings.Join(Names, ", "))
}
