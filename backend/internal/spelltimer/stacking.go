package spelltimer

import (
	"github.com/jasonsoprovich/pq-companion/backend/internal/db"
)

// Buff-overwrite detection, ported from the effect-slot comparison in
// EQMacEmu zone/buffstacking.cpp (Mob::FindAffectSlot and its PatchV1 twin,
// which share this part of the logic).
//
// The server walks the two spells' 12 effect slots in order. The first slot
// where both carry the same (non-ignored) effect id decides the outcome: the
// new buff either overwrites the old one or is blocked. A blank slot in
// either spell ends the walk with no conflict. We only call this after the
// new buff's land message was seen — proof it took hold — so a conflict
// means the old buff was overwritten and is no longer on the recipient.
// That is how Tranquility silently replaces Koadic's Endless Intellect.

// SPA ids (EQMacEmu common/spdat.h) used by the stacking walk.
const (
	seCurrentHP    = 0
	seArmorClass   = 1
	seCHA          = 10
	seCurrentMana  = 15
	seLycanthropy  = 44
	seVampirism    = 45
	seIllusion     = 58
	seHealOverTime = 100
	seCompleteHeal = 101
	seBlank        = 254
)

// stackIgnoredSPAs mirrors BuffStacking::IsSPAIgnoredByStacking: effects that
// never cause a stacking conflict even when both spells carry them.
var stackIgnoredSPAs = map[int]bool{
	13:  true, // SE_SeeInvis
	24:  true, // SE_Stamina
	35:  true, // SE_DiseaseCounter
	36:  true, // SE_PoisonCounter
	57:  true, // SE_Levitate
	65:  true, // SE_InfraVision
	66:  true, // SE_UltraVision
	79:  true, // SE_CurrentHPOnce
	116: true, // SE_CurseCounter
	124: true, 125: true, 126: true, 127: true, 128: true, 129: true,
	130: true, 131: true, 132: true, 133: true, // focus effects
	134: true, 135: true, 136: true, 137: true, 138: true, 139: true,
	140: true, 141: true, 142: true, 143: true, 144: true, // focus limits
	148:     true, // SE_StackingCommand_Block
	149:     true, // SE_StackingCommand_Overwrite
	seBlank: true,
}

// bardCanCast mirrors the server's song test (classes[Bard] != 255). Broader
// than isBardSong: it also covers spells shared with other classes.
func bardCanCast(s *db.Spell) bool {
	return s.ClassLevels[7] < 255
}

func hasSPA(s *db.Spell, spa int) bool {
	for _, id := range s.EffectIDs {
		if id == spa {
			return true
		}
	}
	return false
}

// buffOverwrites reports whether newSp, having landed on a recipient, has
// overwritten oldSp on that same recipient. Only beneficial-over-beneficial
// is modelled; anything the walk can't settle confidently returns false so
// the existing timer is left alone.
//
// Bard songs are skipped outright: whether a song stacks with a non-song
// depends on the caster's class (buffstacking.cpp), which a land line does
// not reveal reliably.
func buffOverwrites(newSp, oldSp *db.Spell) bool {
	if newSp == nil || oldSp == nil || newSp.ID == oldSp.ID {
		return false
	}
	if newSp.GoodEffect == 0 || oldSp.GoodEffect == 0 {
		return false
	}
	if bardCanCast(newSp) || bardCanCast(oldSp) {
		return false
	}
	for i := 0; i < len(newSp.EffectIDs); i++ {
		oldID, newID := oldSp.EffectIDs[i], newSp.EffectIDs[i]
		if oldID == seBlank || newID == seBlank {
			return false // end of either spell: stacks
		}
		if newID == seLycanthropy || newID == seVampirism {
			return false // new buff would have been blocked
		}
		if oldID != newID || stackIgnoredSPAs[newID] {
			continue
		}
		if newID == seCurrentHP || newID == seArmorClass {
			if newSp.EffectBaseValues[i] < 0 {
				continue // DoT / AC debuff component: ignored for stacking
			}
		}
		if newID == seCHA &&
			(newSp.EffectBaseValues[i] == 0 || oldSp.EffectBaseValues[i] == 0) {
			continue // SE_CHA with 0 base is a spacer
		}
		// Conflict in slot i. The server blocks a beneficial buff over an
		// illusion or Donal's BP rather than overwriting it; if we see that
		// combination our view of the recipient is off, so keep the timer.
		if hasSPA(oldSp, seIllusion) || oldSp.EffectIDs[i] == seCompleteHeal {
			return false
		}
		return true
	}
	return false
}

// rememberStackSpell caches a resolved spell so overwrite checks can read an
// existing timer's effect slots while holding e.mu, without a DB query.
func (e *Engine) rememberStackSpell(s *db.Spell) {
	if s == nil || s.ID <= 0 {
		return
	}
	e.stackSpellMu.Lock()
	if e.stackSpells == nil {
		e.stackSpells = make(map[int]*db.Spell)
	}
	e.stackSpells[s.ID] = s
	e.stackSpellMu.Unlock()
}

func (e *Engine) cachedStackSpell(id int) *db.Spell {
	e.stackSpellMu.Lock()
	defer e.stackSpellMu.Unlock()
	return e.stackSpells[id]
}
