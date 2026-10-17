package sapitts

import "testing"

func TestSAPIRate(t *testing.T) {
	tests := []struct {
		rate float64
		want int
	}{
		{1, 0},
		{3, 10},
		{2, 6},
		{0.5, -6},
		{0.1, -10},
		{10, 10},
	}
	for _, tt := range tests {
		if got := sapiRate(tt.rate); got != tt.want {
			t.Errorf("sapiRate(%v) = %d, want %d", tt.rate, got, tt.want)
		}
	}
}

func TestCachePathDistinct(t *testing.T) {
	base := t.TempDir()
	a := cachePath(base, "Microsoft David", 1, "mez broke")
	cases := []string{
		cachePath(base, "Microsoft Zira", 1, "mez broke"),
		cachePath(base, "Microsoft David", 1.5, "mez broke"),
		cachePath(base, "Microsoft David", 1, "charm broke"),
	}
	for i, c := range cases {
		if c == a {
			t.Errorf("case %d: cache path collided with base entry", i)
		}
	}
	if again := cachePath(base, "Microsoft David", 1, "mez broke"); again != a {
		t.Errorf("cache path not stable: %s vs %s", again, a)
	}
}
