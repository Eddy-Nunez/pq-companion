package character

import (
	"reflect"
	"testing"
)

func TestSpellPassRoundTrip(t *testing.T) {
	s, charID := openTestStore(t)

	if got, err := s.ListSpellPasses(charID); err != nil || len(got) != 0 {
		t.Fatalf("empty list = %v, %v", got, err)
	}

	// Adds are idempotent and the list comes back sorted.
	for _, id := range []int{300, 100, 100, 200} {
		if err := s.AddSpellPass(charID, id); err != nil {
			t.Fatalf("add %d: %v", id, err)
		}
	}
	got, err := s.ListSpellPasses(charID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if want := []int{100, 200, 300}; !reflect.DeepEqual(got, want) {
		t.Errorf("after adds = %v, want %v", got, want)
	}

	if err := s.RemoveSpellPass(charID, 200); err != nil {
		t.Fatalf("remove: %v", err)
	}
	// Removing something not passed is a no-op.
	if err := s.RemoveSpellPass(charID, 999); err != nil {
		t.Fatalf("remove missing: %v", err)
	}
	got, _ = s.ListSpellPasses(charID)
	if want := []int{100, 300}; !reflect.DeepEqual(got, want) {
		t.Errorf("after remove = %v, want %v", got, want)
	}
}

func TestSpellPassIsPerCharacter(t *testing.T) {
	s, charID := openTestStore(t)
	other, err := s.Create("Otherchar", 1, 1, 60)
	if err != nil {
		t.Fatalf("create other: %v", err)
	}
	if err := s.AddSpellPass(charID, 42); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got, _ := s.ListSpellPasses(other.ID); len(got) != 0 {
		t.Errorf("other character sees %v, want none", got)
	}
}
