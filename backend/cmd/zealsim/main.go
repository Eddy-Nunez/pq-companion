// Command zealsim is a fake Zeal: it hosts a named pipe following the
// zeal_<pid> convention, lets the backend's Zeal pipe supervisor discover and
// dial it, and feeds synthetic envelopes — MsgPlayer snapshots (zone stamp +
// position) and MsgRaid rosters — on the ~10/sec cadance the real DLL uses.
// The encoder side lives in internal/zealpipe/simulate.go and is covered by
// round-trip tests against the real decoders, so this tool can only drift
// from the wire contract if the contract itself changes.
//
// Usage (Windows; cross-compile from WSL with GOOS=windows):
//
//	zealsim                                  # 24-member demo raid, Kael, 10 ticks/sec
//	zealsim -members 54 -groups 9            # full 54-person raid sheet
//	zealsim -scenario roster.json            # custom roster JSON
//	zealsim -disband-after 30s               # raid for 30s, then silence (stale path)
//	zealsim -once                            # one tick, then exit (scripted probes)
//	zealsim -zone 119 -interval 1s           # Wakening Lands, slow ticks
//
// The default pipe name (zeal_000999) deliberately sorts before any real
// eqgame pipe (Zeal names them zeal_<pid>; PIDs never start with '0'), so a
// pristine test backend attaches to the simulator even while the live client
// is running. Ctrl+C stops the tool; the backend then rediscovers whatever
// pipes remain.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jasonsoprovich/pq-companion/backend/internal/zealpipe"
)

// defaultPipe deliberately sorts before any real eqgame pipe (Zeal names
// them zeal_<pid>; PIDs never start with '0'), so a pristine test backend
// discovers the simulator first even while the live client is running.
const defaultPipe = `\\.\pipe\zeal_000999`

// demoMix is the class mix the generated roster cycles through, in Zeal
// 1-indexed class ids: warriors, clerics, SK, druid, monk, bard, rogue,
// shaman, enchanter, rangers, wizard, magician, necromancer, beastlord,
// paladin — enough tanks/healers/support for trinity grouping plus damage.
var demoClasses = []int{1, 1, 2, 2, 2, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 3, 4}

func main() {
	log.SetFlags(log.Ltime)

	var (
		pipeName  = flag.String("pipe", defaultPipe, "named pipe to host (must glob-match zeal_<pid>)")
		character = flag.String("character", "Testdriver", "character field on every envelope")
		zone      = flag.Int("zone", 113, "zone id the MsgPlayer snapshot reports (113 = Kael Drakkel, 119 = Wakening Lands)")
		members   = flag.Int("members", 24, "demo roster size")
		groups    = flag.Int("groups", 4, "groups to distribute the demo roster across")
		interval  = flag.Duration("interval", 100*time.Millisecond, "tick interval (Zeal emits ~10/sec)")
		disband   = flag.Duration("disband-after", 0, "stop raid envelopes after this long (exercises the backend's disband/stale path)")
		scenario  = flag.String("scenario", "", "JSON file: array of {name, level, class, group, rank}, or {\"zone\":N,\"members\":[...]}")
		once      = flag.Bool("once", false, "send one tick, then close the pipe and exit (scripted probes)")
		quiet     = flag.Bool("quiet", false, "log state changes only, not periodic ticks")
	)
	flag.Parse()

	roster := demoRoster(*members, *groups)
	if path := *scenario; path != "" {
		members, scenarioZone, err := loadScenario(path)
		if err != nil {
			log.Fatalf("zealsim: %v", err)
		}
		if len(members) > 0 {
			roster = members
		}
		if scenarioZone > 0 {
			*zone = scenarioZone
		}
		log.Printf("zealsim: scenario %s loaded (%d members)", path, len(roster))
	}

	if err := serve(*pipeName, *character, *zone, roster, *interval, *disband, *once, *quiet); err != nil {
		log.Fatalf("zealsim: %v", err)
	}
}

// loadScenario parses a scenario file: a bare JSON array of members, or an
// object {"zone":N,"members":[...]} that also overrides the zone flag.
func loadScenario(path string) ([]zealpipe.SimRaidMember, int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	// Editors and PowerShell's UTF8 writer commonly add a BOM; Go's
	// encoding/json rejects it, so strip one if present.
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	var wrapper struct {
		Zone    int                      `json:"zone"`
		Members []zealpipe.SimRaidMember `json:"members"`
	}
	if err := json.Unmarshal(b, &wrapper); err == nil && wrapper.Members != nil {
		return wrapper.Members, wrapper.Zone, nil
	}
	var bare []zealpipe.SimRaidMember
	if err := json.Unmarshal(b, &bare); err != nil {
		return nil, 0, fmt.Errorf("scenario %s: want a member array or {zone, members}: %w", path, err)
	}
	return bare, 0, nil
}

func demoRoster(n, groups int) []zealpipe.SimRaidMember {
	out := make([]zealpipe.SimRaidMember, 0, n)
	for i := 0; i < n; i++ {
		g := i/6 + 1
		if g > groups {
			g = groups
		}
		rank := ""
		switch {
		case i == 0:
			rank = "Raid Leader"
		case i%6 == 0:
			rank = "Group Leader"
		}
		out = append(out, zealpipe.SimRaidMember{
			Name:  fmt.Sprintf("Tester%02d", i+1),
			Level: 55 + i%6,
			Class: demoClasses[i%len(demoClasses)],
			Group: fmt.Sprintf("%d", g),
			Rank:  rank,
		})
	}
	return out
}
