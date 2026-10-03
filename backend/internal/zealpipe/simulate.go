package zealpipe

import (
	"encoding/json"
	"fmt"
)

// This file is the ENCODE side of the pipe protocol, for the zealsim test
// tool: it lets a test scenario emit envelopes shaped exactly like Zeal's
// output (payload double-encoded as a JSON string — see Envelope) so the
// backend consumes them with the same Decode* path real traffic takes.
// The decoder lives in events.go; the simulator's Windows pipe server lives
// in cmd/zealsim. Everything here is platform-independent and covered by
// round-trip tests against the decoders.

// SimRaidMember is one roster entry in a simulated MsgRaid payload. Class is
// the Zeal 1-indexed class id (1=Warrior .. 15=Beastlord), the same numbering
// DecodeRaid's classID expects.
type SimRaidMember struct {
	Name  string `json:"name"`
	Level int    `json:"level"`
	Class int    `json:"class"`
	Group string `json:"group"` // "0" = ungrouped, "1".."12"
	Rank  string `json:"rank,omitempty"`
}

// MarshalRaidEnvelope builds one MsgRaid (type 5) envelope line: the roster
// payload JSON-encoded as a string inside the envelope. Callers write the
// returned bytes back-to-back onto the pipe (json.Decoder on the backend
// side stream-parses values; no framing delimiter is needed, but trailing
// whitespace/newlines are harmless).
func MarshalRaidEnvelope(character string, members []SimRaidMember) ([]byte, error) {
	payload, err := json.Marshal(members)
	if err != nil {
		return nil, fmt.Errorf("zealpipe: marshal raid payload: %w", err)
	}
	return marshalEnvelope(MsgRaid, character, payload)
}

// MarshalPlayerEnvelope wraps a Player snapshot as one MsgPlayer envelope.
func MarshalPlayerEnvelope(character string, p Player) ([]byte, error) {
	payload, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("zealpipe: marshal player payload: %w", err)
	}
	return marshalEnvelope(MsgPlayer, character, payload)
}

// MarshalGroupEnvelope wraps group members (MsgGroup type 6, GroupMember
// array) as one envelope.
func MarshalGroupEnvelope(character string, members []GroupMember) ([]byte, error) {
	payload, err := json.Marshal(members)
	if err != nil {
		return nil, err
	}
	return marshalEnvelope(MsgGroup, character, payload)
}

// marshalEnvelope does the shared double-encode: payload bytes → JSON
// string → envelope object.
func marshalEnvelope(t PipeMessageType, character string, payload []byte) ([]byte, error) {
	env := Envelope{Type: t, Character: character, Data: string(payload), DataLen: len(payload)}
	line, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("zealpipe: marshal envelope: %w", err)
	}
	return line, nil
}
