package main

import "testing"

func TestConsumerGuard(t *testing.T) {
	g := newConsumerGuard()

	var bad, good int
	for i := 0; i < 3; i++ {
		g.run("bad", func() {
			bad++
			var m map[string]int
			m["x"] = 1 // nil map write panics
		})
		g.run("good", func() { good++ })
	}

	if bad != 1 {
		t.Errorf("panicking consumer ran %d times, want 1 (disabled after first panic)", bad)
	}
	if good != 3 {
		t.Errorf("healthy consumer ran %d times, want 3", good)
	}
}
