package config

import "testing"

func TestDecodeOver(t *testing.T) {
	base := func() Config {
		var c Config
		c.ServerAddr = ":1234"
		c.Preferences.NPCOverlayHiddenItemsByCharacter = map[string][]string{
			"osui":  {"ability.14"},
			"bjorn": {"combat.atk"},
		}
		c.Preferences.RaidThreatPlayerMods = map[string]int{"fondue": 20}
		return c
	}

	tests := []struct {
		name  string
		body  string
		check func(t *testing.T, c Config)
	}{
		{
			name: "sent map drops removed keys",
			body: `{"preferences":{"npc_overlay_hidden_items_by_character":{"bjorn":["combat.atk"]}}}`,
			check: func(t *testing.T, c Config) {
				m := c.Preferences.NPCOverlayHiddenItemsByCharacter
				if _, ok := m["osui"]; ok || len(m) != 1 {
					t.Errorf("by-character = %v, want only bjorn", m)
				}
			},
		},
		{
			name: "empty sent map clears it",
			body: `{"preferences":{"raid_threat_player_mods":{}}}`,
			check: func(t *testing.T, c Config) {
				if len(c.Preferences.RaidThreatPlayerMods) != 0 {
					t.Errorf("player mods = %v, want empty", c.Preferences.RaidThreatPlayerMods)
				}
			},
		},
		{
			name: "omitted fields keep current values",
			body: `{"preferences":{}}`,
			check: func(t *testing.T, c Config) {
				if c.ServerAddr != ":1234" {
					t.Errorf("server addr = %q", c.ServerAddr)
				}
				if len(c.Preferences.NPCOverlayHiddenItemsByCharacter) != 2 {
					t.Errorf("by-character = %v, want unchanged", c.Preferences.NPCOverlayHiddenItemsByCharacter)
				}
				if c.Preferences.RaidThreatPlayerMods["fondue"] != 20 {
					t.Errorf("player mods = %v, want unchanged", c.Preferences.RaidThreatPlayerMods)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base()
			if err := DecodeOver(&c, []byte(tt.body)); err != nil {
				t.Fatalf("DecodeOver: %v", err)
			}
			tt.check(t, c)
		})
	}

	t.Run("invalid JSON errors", func(t *testing.T) {
		c := base()
		if err := DecodeOver(&c, []byte(`{`)); err == nil {
			t.Error("want error for invalid JSON")
		}
	})
}
