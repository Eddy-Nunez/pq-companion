// Command zealdump prints the target fields Zeal publishes on its named pipe.
//
// It exists to verify a Zeal build end to end without needing any PQ Companion
// UI wired up: it dials the same pipe the app uses, decodes with the same
// zealpipe package, and reports what the player message (type 3) actually
// carries for the current target.
//
// It is a developer tool. It is not built into or shipped with the app.
//
//	go run ./cmd/zealdump            # summary line per target change
//	go run ./cmd/zealdump -raw       # every player payload, verbatim
//	go run ./cmd/zealdump -every     # every tick, not just on change
//	go run ./cmd/zealdump -file cap  # replay a capture instead of dialing
//
// To keep a capture for later: go run ./cmd/zealdump -raw > capture.txt
//
// Windows only in practice — the named-pipe namespace does not exist
// elsewhere, so on other platforms it exits with "no Zeal pipe found".
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/jasonsoprovich/pq-companion/backend/internal/zealpipe"
)

// targetKeys are the player-message keys that describe the current target.
// Listed in the order we want them printed, not the order Zeal emits them.
var targetKeys = []string{
	"target_id",
	"target_name",
	"target_type",
	"target_level",
	"target_class",
	"target_race",
	"target_loc",
}

// spawnTypeNames maps Zeal's EntityTypes enum (Zeal/game_structures.h:132).
var spawnTypeNames = map[int]string{
	0: "player",
	1: "NPC",
	2: "NPC corpse",
	3: "player corpse",
	4: "unknown",
}

func main() {
	raw := flag.Bool("raw", false, "print every player payload verbatim")
	every := flag.Bool("every", false, "print every tick, not just when the target changes")
	pipeName := flag.String("pipe", "", "pipe to read (default: first one discovered)")
	fromFile := flag.String("file", "", "read a saved capture instead of dialing a pipe")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var conn io.ReadCloser
	if *fromFile != "" {
		f, err := os.Open(*fromFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open %s: %v\n", *fromFile, err)
			os.Exit(1)
		}
		conn = f
		fmt.Printf("replaying %s\n\n", *fromFile)
	} else {
		conn = dialPipe(ctx, *pipeName)
	}
	defer conn.Close()

	run(ctx, conn, *raw, *every)
}

// dialPipe discovers and opens a live Zeal pipe, exiting with a diagnostic if
// there isn't exactly one obvious candidate.
func dialPipe(ctx context.Context, pipeName string) io.ReadCloser {
	name := pipeName
	if name == "" {
		refs, err := zealpipe.Discover()
		if err != nil {
			fmt.Fprintf(os.Stderr, "discover failed: %v\n", err)
			os.Exit(1)
		}
		if len(refs) == 0 {
			fmt.Fprintln(os.Stderr, "no Zeal pipe found — is EQ running with Zeal loaded and sound enabled?")
			os.Exit(1)
		}

		if len(refs) > 1 {
			fmt.Fprintf(os.Stderr, "note: %d pipes found, using the first (PID %d)\n", len(refs), refs[0].PID)
		}
		name = refs[0].Name
	}

	conn, err := zealpipe.Dial(ctx, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial %s: %v\n", name, err)
		os.Exit(1)
	}
	fmt.Printf("connected to %s — target an NPC in game. Ctrl+C to stop.\n\n", name)
	return conn
}

// run stream-decodes envelopes until EOF, error or cancellation, printing a
// line per player message and a coverage checklist at the end.
func run(ctx context.Context, conn io.Reader, raw, every bool) {
	seen := newCoverage()
	last := ""
	dec := json.NewDecoder(conn)
	for {
		if ctx.Err() != nil {
			break
		}
		var env zealpipe.Envelope
		if err := dec.Decode(&env); err != nil {
			if !errors.Is(err, io.EOF) && ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "\nread ended: %v\n", err)
			}
			break
		}
		if env.Type != zealpipe.MsgPlayer {
			continue
		}
		if raw {
			fmt.Println(env.Data)
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(env.Data), &payload); err != nil {
			fmt.Fprintf(os.Stderr, "decode player payload: %v\n", err)
			continue
		}
		seen.observe(payload)
		line := formatPlayer(payload)
		if every || line != last {
			fmt.Println(line)
			last = line
		}
	}
	fmt.Print("\n" + seen.report())
}

// formatPlayer renders one player payload as a single reviewable line. Kept
// free of I/O so it can be tested against captured fixtures.
func formatPlayer(p map[string]any) string {
	if _, ok := p["target_id"]; !ok {
		return "no target"
	}
	var parts []string
	for _, k := range targetKeys {
		v, ok := p[k]
		if !ok {
			continue
		}
		switch k {
		case "target_type":
			n, isNum := toInt(v)
			if isNum {
				if label, known := spawnTypeNames[n]; known {
					parts = append(parts, fmt.Sprintf("type=%d (%s)", n, label))
					continue
				}
			}
			parts = append(parts, fmt.Sprintf("type=%v", v))
		case "target_loc":
			parts = append(parts, "loc="+formatLoc(v))
		default:
			parts = append(parts, strings.TrimPrefix(k, "target_")+"="+fmt.Sprintf("%v", compact(v)))
		}
	}
	missing := missingKeys(p)
	if len(missing) > 0 {
		parts = append(parts, "MISSING["+strings.Join(missing, ",")+"]")
	}
	return strings.Join(parts, " ")
}

// formatLoc renders Zeal's {x,y,z} object. Zeal's x/y are transposed relative
// to the game's /loc order — see zealpipe.Location — so both readings are
// printed to save the reader converting in their head.
func formatLoc(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	x, _ := toFloat(m["x"])
	y, _ := toFloat(m["y"])
	z, _ := toFloat(m["z"])
	return fmt.Sprintf("(%.1f,%.1f,%.1f) [game /loc %.1f,%.1f]", x, y, z, y, x)
}

// missingKeys lists target descriptors absent from a payload that does have a
// target_id — i.e. the Zeal build predates the target-descriptor change.
func missingKeys(p map[string]any) []string {
	var missing []string
	for _, k := range targetKeys {
		if k == "target_id" {
			continue
		}
		if _, ok := p[k]; !ok {
			missing = append(missing, strings.TrimPrefix(k, "target_"))
		}
	}
	return missing
}

func compact(v any) any {
	if f, ok := v.(float64); ok && f == float64(int64(f)) {
		return int64(f)
	}
	return v
}

func toInt(v any) (int, bool) {
	f, ok := toFloat(v)
	return int(f), ok
}

func toFloat(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

// coverage tracks which of the manual test conditions have actually been hit,
// so the operator gets a checklist at exit instead of having to remember.
type coverage struct {
	noTarget   bool
	spawnTypes map[int]bool
	keys       map[string]bool
	names      map[string]bool
}

func newCoverage() *coverage {
	return &coverage{
		spawnTypes: map[int]bool{},
		keys:       map[string]bool{},
		names:      map[string]bool{},
	}
}

func (c *coverage) observe(p map[string]any) {
	if _, ok := p["target_id"]; !ok {
		c.noTarget = true
		return
	}
	for _, k := range targetKeys {
		if _, ok := p[k]; ok {
			c.keys[k] = true
		}
	}
	if n, ok := toInt(p["target_type"]); ok {
		c.spawnTypes[n] = true
	}
	if s, ok := p["target_name"].(string); ok && s != "" {
		c.names[s] = true
	}
}

func (c *coverage) report() string {
	var b strings.Builder
	b.WriteString("---- coverage ----\n")
	b.WriteString(check(c.noTarget) + " saw a tick with no target (no target_* keys)\n")
	for _, k := range targetKeys {
		b.WriteString(check(c.keys[k]) + " saw " + k + "\n")
	}
	for _, t := range []int{0, 1, 2, 3} {
		b.WriteString(check(c.spawnTypes[t]) + fmt.Sprintf(" saw target_type=%d (%s)\n", t, spawnTypeNames[t]))
	}
	if len(c.names) > 0 {
		names := make([]string, 0, len(c.names))
		for n := range c.names {
			names = append(names, n)
		}
		sort.Strings(names)
		b.WriteString("\ndistinct target names seen: " + strings.Join(names, ", ") + "\n")
	}
	return b.String()
}

func check(ok bool) string {
	if ok {
		return "[x]"
	}
	return "[ ]"
}
