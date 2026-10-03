package zealpipe

import (
	"encoding/json"
	"strings"
	"testing"
)

// Round-trip: what the simulator marshals, the real decoders must consume.
// These mirror the live wire fixtures in events_test.go (double-encoded
// payload, flexInt levels, class-by-name tolerance) so a simulator regression
// that breaks the real Zeal contract fails here first.

func TestMarshalRaidEnvelopeRoundTrip(t *testing.T) {
	members := []SimRaidMember{
		{Name: "Tenchi", Level: 60, Class: 12, Group: "1", Rank: "Raid Leader"},
		{Name: "Bonce", Level: 54, Class: 1, Group: "1"},
		{Name: "Cureall", Level: 57, Class: 2, Group: "2", Rank: "Group Leader"},
	}
	line, err := MarshalRaidEnvelope("Osui", members)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	env, err := DecodeEnvelope(line)
	if err != nil {
		t.Fatalf("DecodeEnvelope: %v", err)
	}
	if env.Type != MsgRaid {
		t.Fatalf("Type = %v, want MsgRaid", env.Type)
	}
	if env.Character != "Osui" {
		t.Errorf("Character = %q, want Osui", env.Character)
	}
	got, err := DecodeRaid(env.Data)
	if err != nil {
		t.Fatalf("DecodeRaid: %v", err)
	}
	if len(members) != 3 {
		t.Fatalf("round-trip lost members: %d", len(members))
	}
	if got[0].Name != "Tenchi" || int(members[0].Class) != 12 || int(members[0].Level) != 60 {
		t.Errorf("members[0] = %+v, want Tenchi/60/Wizard(12)", members[0])
	}
	if members[0].Group != "1" || members[0].Rank != "Raid Leader" {
		t.Errorf("members[0] group/rank = %q/%q", members[0].Group, members[0].Rank)
	}
}

func TestMarshalPlayerEnvelopeRoundTrip(t *testing.T) {
	spawn := 12345
	p := Player{
		Zone:       113, // Kael Drakkel
		Location:   Location{X: -100.5, Y: 250.25, Z: 3.0},
		Heading:    128.0,
		AutoAttack: false,
		TargetID:   &spawn,
	}
	line, err := MarshalPlayerEnvelope("Kravija", p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	env, err := DecodeEnvelope(line)
	if err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.Type != MsgPlayer {
		t.Fatalf("Type = %v, want MsgPlayer", env.Type)
	}
	got, err := DecodePlayer(env.Data)
	if err != nil {
		t.Fatalf("decode player: %v", err)
	}
	if got.Zone != 113 || got.Location.GameX() != 250.25 || got.Location.GameY() != -100.5 {
		t.Errorf("player round-trip mismatch: %+v", got)
	}
}

func TestMarshalEnvelopeFraming(t *testing.T) {
	// Two envelopes written back-to-back (no delimiter) must stream-decode
	// exactly like the backend's readLoop does.
	l1, err := MarshalRaidEnvelope("Osui", []SimRaidMember{{Name: "A", Level: 1, Class: 1, Group: "1"}})
	if err != nil {
		t.Fatal(err)
	}
	l2, err := MarshalPlayerEnvelope("Osui", Player{Zone: 113})
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(strings.NewReader(string(l1) + string(l2)))
	var env Envelope
	if err := dec.Decode(&env); err != nil || env.Type != MsgRaid {
		t.Fatalf("first envelope: %v type=%v", err, env.Type)
	}
	if err := dec.Decode(&env); err != nil || env.Type != MsgPlayer {
		t.Fatalf("second envelope: %v type=%v", err, env.Type)
	}
}
