package blockbuff

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var testSpells = map[int]string{
	278:  "Spirit of Wolf",
	1397: "Pegasus Feather Cloak (Test)",
	2:    "Clarity",
}

func testNames(id int) string { return testSpells[id] }

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "user.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want Line
	}{
		{"added", "Blocking Spirit of Wolf (278) from other players.",
			Line{Kind: KindAdded, Entry: Entry{278, 0}, Name: "Spirit of Wolf"}},
		{"added conditional", "Blocking Spirit of Wolf (278) from other players while Clarity (2) is on you.",
			Line{Kind: KindAdded, Entry: Entry{278, 2}, Name: "Spirit of Wolf", IfName: "Clarity", hasIfName: true}},
		{"removed", "No longer blocking Spirit of Wolf (278).",
			Line{Kind: KindRemoved, Entry: Entry{278, 0}, Name: "Spirit of Wolf"}},
		{"removed conditional", "No longer blocking Spirit of Wolf (278) while Clarity (2) is on you.",
			Line{Kind: KindRemoved, Entry: Entry{278, 2}, Name: "Spirit of Wolf", IfName: "Clarity", hasIfName: true}},
		{"absent", "You are not blocking Spirit of Wolf (278).",
			Line{Kind: KindAbsent, Entry: Entry{278, 0}, Name: "Spirit of Wolf"}},
		{"empty", "You are not blocking any buffs.", Line{Kind: KindEmpty}},
		{"list row", "Spirit of Wolf (278)",
			Line{Kind: KindListRow, Entry: Entry{278, 0}, Name: "Spirit of Wolf"}},
		{"list row conditional", "Spirit of Wolf (278) while Clarity (2) is on you",
			Line{Kind: KindListRow, Entry: Entry{278, 2}, Name: "Spirit of Wolf", IfName: "Clarity", hasIfName: true}},
		{"parenthesised spell name", "Pegasus Feather Cloak (Test) (1397)",
			Line{Kind: KindListRow, Entry: Entry{1397, 0}, Name: "Pegasus Feather Cloak (Test)"}},
		{"look-alike with wrong name", "A gnoll (278)", Line{}},
		{"look-alike with unknown id", "Spirit of Wolf (99999)", Line{}},
		{"condition name mismatch", "Spirit of Wolf (278) while Nope (2) is on you", Line{}},
		{"unrelated", "You slash a gnoll for 150 points of damage.", Line{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Parse(tc.msg, testNames); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q) = %+v, want %+v", tc.msg, got, tc.want)
			}
		})
	}
	// Without a name check, bare list rows must never be trusted.
	if got := Parse("Spirit of Wolf (278)", nil); got.Kind != KindNone {
		t.Errorf("nil NameCheck accepted a list row: %+v", got)
	}
}

func TestReconcileAndCommands(t *testing.T) {
	desired := []Entry{{278, 0}, {2, 0}, {278, 2}}
	observed := []Entry{{278, 0}, {1397, 0}}

	rows := Reconcile(desired, observed, true)
	got := map[Entry]string{}
	for _, r := range rows {
		got[r.Entry] = r.Status
	}
	want := map[Entry]string{
		{278, 0}:  StatusApplied,
		{2, 0}:    StatusNeedsBlock,
		{278, 2}:  StatusNeedsBlock,
		{1397, 0}: StatusNeedsAllow,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("synced statuses = %v, want %v", got, want)
	}
	wantCmds := []string{"#blockbuff 2", "#blockbuffif 278 2", "#allowbuff 1397"}
	if cmds := Commands(rows); !reflect.DeepEqual(cmds, wantCmds) {
		t.Errorf("commands = %v, want %v", cmds, wantCmds)
	}

	// Unsynced: absence proves nothing, so no #allowbuff and wanted rows are
	// "unconfirmed" (re-blocking is harmless server-side).
	rows = Reconcile(desired, []Entry{{1397, 0}}, false)
	for _, r := range rows {
		if r.Desired && r.Status != StatusUnconfirmed {
			t.Errorf("%v status = %s, want unconfirmed", r.Entry, r.Status)
		}
	}
	for _, c := range Commands(rows) {
		if c == "#allowbuff 1397" {
			t.Errorf("suggested #allowbuff without a full server listing")
		}
	}

	if got := Command(Entry{5, 7}, false); got != "#allowbuff 5 7" {
		t.Errorf("conditional allow = %q", got)
	}
}

func TestMachineLiveFlow(t *testing.T) {
	s := openTestStore(t)
	m := NewMachine(s, testNames)
	base := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	at := func(sec int) time.Time { return base.Add(time.Duration(sec) * time.Second) }

	m.Feed("Bob", at(0), "Blocking Spirit of Wolf (278) from other players.")
	obs, _ := s.Observed("Bob")
	if !reflect.DeepEqual(obs, []Entry{{278, 0}}) {
		t.Fatalf("after add observed = %v", obs)
	}
	if v, _ := s.SyncedAt("Bob"); v != 0 {
		t.Errorf("a single add must not mark the server list as synced (got %d)", v)
	}

	// A listing replaces everything and marks the character synced; it commits
	// when the next non-list line arrives.
	m.Feed("Bob", at(10), "Clarity (2)")
	m.Feed("Bob", at(10), "Spirit of Wolf (278) while Clarity (2) is on you")
	if !m.Pending() {
		t.Fatal("listing rows should be buffered")
	}
	if !m.Feed("Bob", at(11), "You slash a gnoll for 5 points of damage.") {
		t.Error("flush on a following line should report a change")
	}
	obs, _ = s.Observed("Bob")
	if !reflect.DeepEqual(obs, []Entry{{2, 0}, {278, 2}}) {
		t.Fatalf("after listing observed = %v", obs)
	}
	if v, _ := s.SyncedAt("Bob"); v != at(10).Unix() {
		t.Errorf("synced_at = %d, want %d", v, at(10).Unix())
	}

	m.Feed("Bob", at(20), "No longer blocking Clarity (2).")
	m.Feed("Bob", at(21), "You are not blocking any buffs.")
	obs, _ = s.Observed("Bob")
	if len(obs) != 0 {
		t.Errorf("empty reply should clear observed, got %v", obs)
	}
}

func TestBackfillDoesNotRegressNewerData(t *testing.T) {
	s := openTestStore(t)
	base := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

	// Live session recorded a newer state.
	live := NewMachine(s, testNames)
	live.Feed("Bob", base.Add(time.Hour), "Blocking Clarity (2) from other players.")

	// Backfilling an older log (which blocked Spirit of Wolf, then listed an
	// empty set) must not clobber it.
	h := NewBackfillHandler(s, testNames, "Bob")
	h.HandleLine(base, "Blocking Spirit of Wolf (278) from other players.")
	h.HandleLine(base.Add(time.Minute), "You are not blocking any buffs.")
	h.Finalize()

	obs, _ := s.Observed("Bob")
	if !reflect.DeepEqual(obs, []Entry{{2, 0}}) {
		t.Fatalf("observed = %v, want only the newer live block", obs)
	}
	if h.Inserted() != 0 {
		t.Errorf("Inserted = %d, want 0 for fully stale replay", h.Inserted())
	}
}

func TestStoreDesiredAndAdopt(t *testing.T) {
	s := openTestStore(t)
	if err := s.SetDesired("Bob", []Entry{{278, 0}, {278, 0}, {2, 0}}); err != nil {
		t.Fatal(err)
	}
	d, _ := s.Desired("Bob")
	if !reflect.DeepEqual(d, []Entry{{2, 0}, {278, 0}}) {
		t.Fatalf("desired = %v (duplicates must collapse)", d)
	}
	s.ObserveSnapshot("Bob", []Entry{{1397, 0}}, 100)
	if err := s.AdoptObserved("Bob"); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Desired("Bob")
	if !reflect.DeepEqual(d, []Entry{{1397, 0}}) {
		t.Errorf("adopt: desired = %v, want server list", d)
	}
}
