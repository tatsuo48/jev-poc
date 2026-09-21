package player

import "testing"

func TestNewBuildsEveryNamedPlayer(t *testing.T) {
	for _, name := range Names {
		p, err := New(name, 1, &fakeAsker{})
		if err != nil || p.Name() != name {
			t.Errorf("New(%q) = %v, %v", name, p, err)
		}
	}
}

func TestNewRejectsUnknownName(t *testing.T) {
	if _, err := New("minimax", 1, nil); err == nil {
		t.Fatal("expected an error")
	}
}

func TestNewRequiresAskerForJevPlayers(t *testing.T) {
	for _, name := range []string{"jev-raw", "jev-sim"} {
		if !NeedsJev(name) {
			t.Errorf("NeedsJev(%q) = false", name)
		}
		if _, err := New(name, 1, nil); err == nil {
			t.Errorf("New(%q) without an asker succeeded", name)
		}
	}
	if NeedsJev("greedy") {
		t.Error(`NeedsJev("greedy") = true`)
	}
}
