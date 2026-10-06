package blockbuff

import "time"

// FlushIdle is how long a #blockbuff listing may sit with no further rows
// before it is committed as a full snapshot. A listing arrives in one tight
// burst, so this just needs to exceed the natural line spacing.
const FlushIdle = 1500 * time.Millisecond

// Machine classifies log lines for one character and writes what the server
// confirmed to the Store. It is shared by the live Consumer and the log
// BackfillHandler; neither adds locking, timers or broadcasting here.
type Machine struct {
	store *Store
	names NameCheck

	rows []Entry   // listing rows buffered since the burst started
	at   time.Time // log time of the newest buffered row
}

// NewMachine builds a Machine. names may be nil in tests that only use reply
// lines (list rows are then ignored).
func NewMachine(store *Store, names NameCheck) *Machine {
	return &Machine{store: store, names: names}
}

// Pending reports whether listing rows are buffered awaiting a flush.
func (m *Machine) Pending() bool { return len(m.rows) > 0 }

// Feed processes one line for character and returns whether stored data
// changed. A line that isn't part of a listing first flushes any buffered one.
func (m *Machine) Feed(character string, ts time.Time, msg string) (changed bool) {
	l := Parse(msg, m.names)
	if l.Kind != KindListRow {
		changed = m.Flush(character)
	}
	switch l.Kind {
	case KindListRow:
		m.rows = append(m.rows, l.Entry)
		m.at = ts
	case KindAdded:
		ok, _ := m.store.ObserveAdded(character, l.Entry, ts.Unix())
		changed = changed || ok
	case KindRemoved, KindAbsent:
		ok, _ := m.store.ObserveRemoved(character, l.Entry, ts.Unix())
		changed = changed || ok
	case KindEmpty:
		ok, _ := m.store.ObserveSnapshot(character, nil, ts.Unix())
		changed = changed || ok
	}
	return changed
}

// Flush commits buffered listing rows as a full server snapshot.
func (m *Machine) Flush(character string) bool {
	if len(m.rows) == 0 {
		return false
	}
	rows, at := m.rows, m.at
	m.rows = nil
	if character == "" {
		return false
	}
	ok, _ := m.store.ObserveSnapshot(character, rows, at.Unix())
	return ok
}
