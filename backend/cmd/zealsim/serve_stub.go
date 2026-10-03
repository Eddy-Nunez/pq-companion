//go:build !windows

package main

import (
	"errors"
	"time"

	"github.com/jasonsoprovich/pq-companion/backend/internal/zealpipe"
)

// serve is the non-Windows stub: the simulator hosts a Windows named pipe
// (Zeal runs inside the EQ client on Windows), so there is nothing to do on
// a dev box — the message matches the client.go stub's macOS note.
func serve(pipeName, character string, zone int, roster []zealpipe.SimRaidMember, interval, disbandAfter time.Duration, once, quiet bool) error {
	return errors.New("zealsim: Windows only — named pipes are a Windows facility")
}
