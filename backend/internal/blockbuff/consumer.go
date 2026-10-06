package blockbuff

import (
	"sync"
	"time"
)

// Consumer feeds live log lines to a Machine for the active character and
// reports changes so an open Blocked Buffs page can refetch.
type Consumer struct {
	activeChar func() string
	onChange   func(character string)

	mu        sync.Mutex
	machine   *Machine
	character string // character the buffered listing belongs to
	timer     *time.Timer
}

// NewConsumer wires a consumer to store. names resolves spell ids to names
// (see NameCheck); activeChar returns the in-game character, "" to ignore.
func NewConsumer(store *Store, names NameCheck, activeChar func() string) *Consumer {
	return &Consumer{machine: NewMachine(store, names), activeChar: activeChar}
}

// SetOnChange registers a callback fired after stored data changes.
func (c *Consumer) SetOnChange(fn func(character string)) {
	c.mu.Lock()
	c.onChange = fn
	c.mu.Unlock()
}

// HandleLine processes one raw log line (text after the timestamp).
func (c *Consumer) HandleLine(ts time.Time, msg string) {
	if msg == "" || c.activeChar == nil {
		return
	}
	char := c.activeChar()
	if char == "" {
		return
	}
	c.mu.Lock()
	// A listing buffered under another character is committed to that one
	// before switching, never to the new one.
	changedChar := ""
	if c.machine.Pending() && c.character != "" && c.character != char {
		if c.machine.Flush(c.character) {
			changedChar = c.character
		}
	}
	changed := c.machine.Feed(char, ts, msg)
	if c.machine.Pending() {
		c.character = char
		c.armLocked()
	} else if c.timer != nil {
		c.timer.Stop()
	}
	cb := c.onChange
	c.mu.Unlock()

	if cb == nil {
		return
	}
	if changedChar != "" {
		cb(changedChar)
	}
	if changed {
		cb(char)
	}
}

// armLocked (re)starts the idle timer that commits a trailing listing.
func (c *Consumer) armLocked() {
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(FlushIdle, func() {
		c.mu.Lock()
		char := c.character
		changed := c.machine.Flush(char)
		cb := c.onChange
		c.mu.Unlock()
		if changed && cb != nil {
			cb(char)
		}
	})
}
