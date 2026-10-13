package killtracker

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jasonsoprovich/pq-companion/backend/internal/logparser"
	"github.com/jasonsoprovich/pq-companion/backend/internal/trigger"
	"github.com/jasonsoprovich/pq-companion/backend/internal/ws"
)

type fakeHub struct {
	mu     sync.Mutex
	events []ws.Event
}

func (f *fakeHub) Broadcast(ev ws.Event) {
	f.mu.Lock()
	f.events = append(f.events, ev)
	f.mu.Unlock()
}

// alerts returns the text of every synthetic trigger:fired event.
func (f *fakeHub) alerts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, ev := range f.events {
		if ev.Type == trigger.WSEventTriggerFired {
			out = append(out, ev.Data.(trigger.TriggerFired).MatchedLine)
		}
	}
	return out
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "user.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

var t0 = time.Date(2026, 10, 13, 20, 0, 0, 0, time.UTC)

func killEv(at time.Time, target, killer string) logparser.LogEvent {
	return logparser.LogEvent{Type: logparser.EventKill, Timestamp: at, Data: logparser.KillData{Target: target, Killer: killer}}
}

func zoneEv(at time.Time, zone string) logparser.LogEvent {
	return logparser.LogEvent{Type: logparser.EventZone, Timestamp: at, Data: logparser.ZoneData{ZoneName: zone}}
}

func TestIsProperName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Darwol Adan", true},
		{"Grummus", true},
		{"a gnoll", false},
		{"an owlbear", false},
		{"A Shissar Wrath", false},
		{"An Ancient Wyvern", false},
		{"Knight of Decay", true}, // known false positive, documented
		{"", false},
	}
	for _, tt := range tests {
		if got := isProperName(tt.name); got != tt.want {
			t.Errorf("isProperName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestKillLogDedupAndSeq(t *testing.T) {
	s := openTestStore(t)
	players := map[string]bool{"osui": true}
	classify := func(n string) bool { return !players[strings.ToLower(n)] }

	feed := func(h interface{ HandleEvent(logparser.LogEvent) }) {
		h.HandleEvent(zoneEv(t0, "The Crypt of Decay"))
		// AE pull: two same-name kills by the same killer in one second.
		h.HandleEvent(killEv(t0.Add(time.Second), "Knight of Decay", "Ineka"))
		h.HandleEvent(killEv(t0.Add(time.Second), "Knight of Decay", "Ineka"))
		h.HandleEvent(killEv(t0.Add(2*time.Second), "Darwol Adan", "You"))
		h.HandleEvent(killEv(t0.Add(3*time.Second), "Osui", "Darwol Adan"))
	}

	live := NewLogger(s, classify, func() string { return "Osui" })
	var got []Entry
	live.SetOnInsert(func(e Entry) { got = append(got, e) })
	feed(live)
	if len(got) != 4 {
		t.Fatalf("live inserts = %d, want 4", len(got))
	}

	// A backfill of the same log must not duplicate anything.
	bf := NewBackfillHandler(s, classify, "Osui")
	feed(bf)
	if bf.Inserted() != 0 {
		t.Fatalf("backfill inserted %d duplicates", bf.Inserted())
	}

	npc, err := s.List(Filters{Character: "Osui"})
	if err != nil {
		t.Fatal(err)
	}
	if len(npc) != 3 {
		t.Fatalf("npc scope rows = %d, want 3 (player death excluded)", len(npc))
	}
	all, _ := s.List(Filters{Character: "Osui", Scope: ScopeAll})
	if len(all) != 4 {
		t.Fatalf("all scope rows = %d, want 4", len(all))
	}
	named, _ := s.List(Filters{Character: "Osui", Scope: ScopeNamed, SortDesc: true})
	if len(named) != 3 || named[0].Target != "Darwol Adan" || named[0].Killer != "Osui" {
		t.Fatalf("named rows = %+v", named)
	}
	if named[0].Zone != "The Crypt of Decay" {
		t.Errorf("zone = %q", named[0].Zone)
	}
}

func newTestCounter(t *testing.T, e *Engine, def CounterDef) Counter {
	t.Helper()
	def, err := Validate(def)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	c, err := e.Create(def)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return c
}

func countOf(e *Engine, id string) Counter {
	for _, c := range e.State().Counters {
		if c.Def.ID == id {
			return c
		}
	}
	return Counter{}
}

func TestCounterKillsZoneGate(t *testing.T) {
	e := NewEngine(nil, &fakeHub{})
	c := newTestCounter(t, e, CounterDef{
		Name: "Knights", Enabled: true, Mobs: []string{"knight of decay"}, Zone: "The Crypt of Decay",
	})

	// Unknown zone still counts (app restarted mid-raid).
	e.Handle(killEv(t0, "Knight of Decay", "x"))
	e.Handle(zoneEv(t0, "The Plane of Knowledge"))
	e.Handle(killEv(t0, "Knight of Decay", "x"))
	e.Handle(zoneEv(t0, "The Crypt of Decay"))
	e.Handle(killEv(t0, "Knight of Decay", "x"))
	e.Handle(killEv(t0, "a gnoll", "x"))

	if got := countOf(e, c.Def.ID).Count; got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
}

func TestCounterPatternSource(t *testing.T) {
	e := NewEngine(nil, nil)
	c := newTestCounter(t, e, CounterDef{Name: "XP", Enabled: true, Pattern: `^You gain (party |raid )?experience`})
	e.HandleLine(t0, "You gain raid experience!!")
	e.HandleLine(t0, "You gain experience!!")
	e.HandleLine(t0, "Osui tells the raid, 'You gain experience'")
	if got := countOf(e, c.Def.ID).Count; got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
}

func TestCounterDisabledIgnored(t *testing.T) {
	e := NewEngine(nil, nil)
	c := newTestCounter(t, e, CounterDef{Name: "X", Enabled: false, Mobs: []string{"a gnoll"}})
	e.Handle(killEv(t0, "a gnoll", ""))
	if got := countOf(e, c.Def.ID).Count; got != 0 {
		t.Fatalf("disabled counter counted: %d", got)
	}
}

func TestValidateRejects(t *testing.T) {
	if _, err := Validate(CounterDef{Name: "", Mobs: []string{"x"}}); err == nil {
		t.Error("empty name accepted")
	}
	if _, err := Validate(CounterDef{Name: "x"}); err == nil {
		t.Error("counter with no source accepted")
	}
	if _, err := Validate(CounterDef{Name: "x", Pattern: "("}); err == nil {
		t.Error("bad regex accepted")
	}
	def, err := Validate(CounterDef{Name: " x ", Mobs: []string{"a", " A ", ""}, Milestones: []Milestone{{At: 9}, {At: 0}, {At: 3}}})
	if err != nil {
		t.Fatal(err)
	}
	if def.Name != "x" || len(def.Mobs) != 1 || len(def.Milestones) != 2 || def.Milestones[0].At != 3 {
		t.Errorf("normalized def = %+v", def)
	}
}

func installBertox(t *testing.T, e *Engine) (trash, kings Counter) {
	t.Helper()
	cs, err := e.InstallPreset("bertox")
	if err != nil {
		t.Fatalf("InstallPreset: %v", err)
	}
	if len(cs) != 2 {
		t.Fatalf("preset counters = %d", len(cs))
	}
	return cs[0], cs[1]
}

func TestBertoxPresetFlow(t *testing.T) {
	hub := &fakeHub{}
	e := NewEngine(nil, hub)
	trash, kings := installBertox(t, e)
	if !e.HasPreset("bertox") {
		t.Fatal("HasPreset false after install")
	}

	e.Handle(zoneEv(t0, "The Crypt of Decay"))
	// Stale count from a previous attempt is wiped by the Spectre emote,
	// which also starts the clock.
	_ = e.SetCount(trash.Def.ID, 17)
	e.HandleLine(t0, "Crazed laughter is heard as you notice a foul creature standing before you. The creature then speaks saying, 'Violaters...'")
	got := countOf(e, trash.Def.ID)
	if got.Count != 0 || got.ClockStartedAt != t0.Unix() {
		t.Fatalf("after spectre: count=%d clock=%d", got.Count, got.ClockStartedAt)
	}

	// 42 visible kills → heads-up for Darwol at 44.
	for i := 0; i < 42; i++ {
		e.Handle(killEv(t0.Add(time.Duration(400+i)*time.Second), "Elite Priest of Decay", "x"))
	}
	if a := hub.alerts(); len(a) != 1 || a[0] != "Darwol Adan spawns in 2 kills" {
		t.Fatalf("alerts after 42 = %q", a)
	}

	// Missed a few kills in another wing; the Feig emote resyncs to 48 and
	// announces the highest milestone crossed.
	e.HandleLine(t0, "An unsettling feeling of fear passes through you as you hear the summoners finish a dark incantation then cry out saying, 'We call to you corrupted King of Lxanvom, Feig Adan, your master has need of you!' A bestial squeak thunders through the crypt as a foul fiend of Bertoxxulous is summoned forth.")
	if got := countOf(e, trash.Def.ID).Count; got != 48 {
		t.Fatalf("after Feig emote count = %d, want 48", got)
	}
	a := hub.alerts()
	if a[len(a)-1] != "Feig Adan spawns (48)" {
		t.Fatalf("last alert = %q", a[len(a)-1])
	}

	// The 8th king death is announced; kings resync to 8.
	e.Handle(killEv(t0, "Darwol Adan", "x"))
	e.HandleLine(t0, "An unsettling feeling of fear passes through you as you hear the summoners finish a dark incantation then cry out saying, 'We call to you the last corrupted Kings of Lxanvom. Meedo Adan! Qezzin Adan! Pzo Adan! Bhaly Adan! Your master has need of you!'")
	if got := countOf(e, kings.Def.ID).Count; got != 8 {
		t.Fatalf("kings = %d, want 8", got)
	}

	// The expiry warning pins the clock even if the start was missed.
	warnAt := t0.Add(2 * time.Hour)
	e.HandleLine(warnAt, "Dark voices whisper in your ear saying, 'Your time is close in coming to an end. Time to flee little ones!'")
	if got := countOf(e, trash.Def.ID).ClockStartedAt; got != warnAt.Add(-7080*time.Second).Unix() {
		t.Fatalf("clock after warning = %d", got)
	}

	// Bertox's death stops the clock without touching counts.
	e.HandleLine(warnAt, "A nimbus of light floods throughs the crypt in one magnificent wave as an earth shattering howl is heard.  The bringer of plagues, lord of all disease and decay, Bertoxxulous has been defeated.")
	got = countOf(e, trash.Def.ID)
	if got.ClockStartedAt != 0 || got.Count != 48 {
		t.Fatalf("after bertox death: %+v", got)
	}
}

func TestCountersPersist(t *testing.T) {
	s := openTestStore(t)
	e := NewEngine(s, nil)
	trash, _ := installBertox(t, e)
	e.Handle(killEv(t0, "Knight of Decay", ""))
	e.Handle(killEv(t0, "Knight of Decay", ""))

	e2 := NewEngine(s, nil)
	st := e2.State()
	if len(st.Counters) != 2 {
		t.Fatalf("reloaded %d counters", len(st.Counters))
	}
	if st.Counters[0].Def.ID != trash.Def.ID || st.Counters[0].Count != 2 {
		t.Fatalf("reloaded first counter = %+v", st.Counters[0])
	}
	if err := e2.Delete(trash.Def.ID); err != nil {
		t.Fatal(err)
	}
	if n := len(NewEngine(s, nil).State().Counters); n != 1 {
		t.Fatalf("after delete reload = %d", n)
	}
}
