package trigger

import (
	"regexp"
	"testing"
)

func TestResolveTimerKeyTemplates(t *testing.T) {
	re := regexp.MustCompile(`^(.+) feels lethargic\. \((.+) \| (\d+%)\)$`)
	line := "A gantru shojen feels lethargic. (Sha's Lethargy | 30%)"
	m := re.FindStringSubmatch(line)
	names := re.SubexpNames()

	cases := []struct {
		name, field, want string
	}{
		{"bare number", "2", "Sha's Lethargy"},
		{"two groups", "{2} {3}", "Sha's Lethargy 30%"},
		{"gina aliases", "{S1}: {S2}", "A gantru shojen: Sha's Lethargy"},
		{"dollar", "$2", "Sha's Lethargy"},
		{"unresolved falls back", "9", "fallback"},
		{"unresolved template falls back", "{9}", "fallback"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tr := &Trigger{TimerKeyCapture: c.field}
			if got := resolveTimerKey(tr, "fallback", m, names); got != c.want {
				t.Errorf("resolveTimerKey(%q) = %q, want %q", c.field, got, c.want)
			}
		})
	}

	t.Run("target template", func(t *testing.T) {
		tr := &Trigger{TimerTargetCapture: "{1}"}
		if got := resolveTimerTarget(tr, m, names); got != "A gantru shojen" {
			t.Errorf("resolveTimerTarget = %q", got)
		}
	})
}
