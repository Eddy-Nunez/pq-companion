//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/jasonsoprovich/pq-companion/backend/internal/zealpipe"
)

// serve hosts the simulated Zeal pipe and serves envelope sessions until the
// context is cancelled (Ctrl+C) or -once mode completes its single tick.
func serve(pipeName, character string, zone int, roster []zealpipe.SimRaidMember, interval, disbandAfter time.Duration, once, quiet bool) error {
	lis, err := winio.ListenPipe(pipeName, nil)
	if err != nil {
		return fmt.Errorf("listen %s: %w", pipeName, err)
	}
	defer lis.Close()
	log.Printf("zealsim: pipe listening — waiting for the backend to dial")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		lis.Close()
	}()

	for {
		conn, err := lis.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil // interrupted — clean exit
			}
			return fmt.Errorf("accept: %w", err)
		}
		if err := pump(ctx, conn, character, zone, roster, interval, disbandAfter, once, quiet); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("zealsim: session error: %v", err)
		} else {
			log.Printf("zealsim: session ended")
		}
		if once || ctx.Err() != nil {
			return nil
		}
	}
}

// pump writes one MsgPlayer + MsgRaid pair per tick. The first pair goes out
// immediately (so -once probes resolve fast); in once mode the connection
// closes after a short drain delay. With -disband-after, raid envelopes stop
// after that point while player snapshots keep flowing — exactly how the
// real client behaves on disband (it simply stops emitting MsgRaid), which
// exercises the backend's 30-second roster-staleness path end to end.
func pump(ctx context.Context, conn io.ReadWriteCloser, character string, zone int, roster []zealpipe.SimRaidMember, interval, disbandAfter time.Duration, once, quiet bool) error {
	defer conn.Close()

	playerLine, err := zealpipe.MarshalPlayerEnvelope(character, zealpipe.Player{Zone: zone})
	if err != nil {
		return err
	}
	raidLine, err := zealpipe.MarshalRaidEnvelope(character, roster)
	if err != nil {
		return err
	}

	// First tick immediately, so -once probes resolve fast.
	if _, err := conn.Write(playerLine); err != nil {
		return err
	}
	if _, err := conn.Write(raidLine); err != nil {
		return err
	}
	log.Printf("zealsim: sent player (zone=%d) + raid (%d members)", zone, len(roster))
	if once {
		time.Sleep(500 * time.Millisecond) // let the backend drain before EOF
		return nil
	}

	start := time.Now()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for tick := 1; ; tick++ {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		if _, err := conn.Write(playerLine); err != nil {
			return err
		}
		inRaid := disbandAfter <= 0 || time.Since(start) < disbandAfter
		if inRaid {
			if _, err := conn.Write(raidLine); err != nil {
				return err
			}
		}
		if !quiet && tick%100 == 0 { // ~every 5s at the default cadence
			log.Printf("zealsim: ticking (members=%d elapsed=%s)", len(roster), time.Since(start).Round(time.Second))
		}
	}
}
