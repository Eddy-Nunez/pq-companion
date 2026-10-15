package spelltimer

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jasonsoprovich/pq-companion/backend/internal/db"
)

// PlayerInfoProvider looks up another player's class index (0–14) and level
// from the /who sighting history. ok=false when the player was never seen
// (or only anonymously with no manual override).
type PlayerInfoProvider func(name string) (class, level int, ok bool)

// reOtherBeginsCast matches the bystander cast-start line. EQ never names the
// spell here, only the caster — the spell is inferred later by pairing this
// with a land message (see otherCasterLevel).
var reOtherBeginsCast = regexp.MustCompile(`^([A-Z][a-z]{2,14}) begins to cast a spell\.$`)

// otherCastRetention bounds how long a cast-start stays in castStarts. Longer
// than any player spell's cast time (Quarm's longest are ~10s, before spell
// haste) plus the attribution slack below.
const otherCastRetention = 20 * time.Second

// Attribution window around the spell's nominal cast time. Spell haste can
// cut a cast up to 50%, and log timestamps only have 1s resolution, so the
// window runs from half the cast time (minus slack) to the full cast time
// plus slack for server/log latency.
const (
	castWindowEarlySlack = 1500 * time.Millisecond
	castWindowLateSlack  = 3 * time.Second
)

// otherCasts records recent "<Name> begins to cast a spell." lines so a later
// land message can be attributed to its caster. It has its own lock: it's fed
// from the raw-line dispatcher and read from the parsed-event path, and never
// needs Engine.mu.
type otherCasts struct {
	mu     sync.Mutex
	starts map[string]time.Time // lowercased caster name → cast start
}

// HandleLine records other players' cast-start lines. Called for every raw
// log line, ahead of the parsed-event dispatch for the same line.
func (e *Engine) HandleLine(ts time.Time, msg string) {
	m := reOtherBeginsCast.FindStringSubmatch(msg)
	if m == nil {
		return
	}
	e.others.mu.Lock()
	defer e.others.mu.Unlock()
	if e.others.starts == nil {
		e.others.starts = make(map[string]time.Time)
	}
	for name, at := range e.others.starts {
		if ts.Sub(at) > otherCastRetention {
			delete(e.others.starts, name)
		}
	}
	e.others.starts[strings.ToLower(m[1])] = ts
}

// SetPlayerInfoProvider wires the /who history lookup used to learn other
// casters' levels. nil (the default) leaves other players at
// defaultCasterLevel.
func (e *Engine) SetPlayerInfoProvider(fn PlayerInfoProvider) {
	e.mu.Lock()
	e.playerInfoFn = fn
	e.mu.Unlock()
}

// otherCasterLevel returns the level of whoever cast spell onto target, when
// it can be pinned down from /who history:
//
//   - Self-only spells: the target is the caster.
//   - Anything else: the players whose "begins to cast a spell." line fits
//     the spell's cast time before landedAt, narrowed to those whose known
//     class can cast spell at their known level. Accepted only when every
//     remaining candidate is known and they all agree on a level — an
//     unknown candidate could be the real caster, so it makes the answer
//     ambiguous rather than being skipped.
func (e *Engine) otherCasterLevel(spell *db.Spell, target string, landedAt time.Time) (int, bool) {
	info := e.playerInfoFn
	if info == nil {
		return 0, false
	}
	if spell.TargetType == targetTypeSelf {
		if target == "" {
			return 0, false
		}
		if class, level, ok := info(target); ok && canCastAt(spell, class, level) {
			return level, true
		}
		return 0, false
	}

	castTime := time.Duration(spell.CastTime) * time.Millisecond
	earliest := landedAt.Add(-castTime - castWindowLateSlack)
	latest := landedAt.Add(-castTime/2 + castWindowEarlySlack)

	e.others.mu.Lock()
	var names []string
	for name, at := range e.others.starts {
		if !at.Before(earliest) && !at.After(latest) {
			names = append(names, name)
		}
	}
	e.others.mu.Unlock()

	level := 0
	for _, name := range names {
		class, lvl, ok := info(name)
		if !ok {
			return 0, false
		}
		if !canCastAt(spell, class, lvl) {
			continue
		}
		if level != 0 && lvl != level {
			return 0, false
		}
		level = lvl
	}
	return level, level > 0
}

// canCastAt reports whether a character of class index class and the given
// level has spell available.
func canCastAt(spell *db.Spell, class, level int) bool {
	if class < 0 || class >= len(spell.ClassLevels) || level <= 0 {
		return false
	}
	need := spell.ClassLevels[class]
	return need > 0 && need < 255 && need <= level
}
