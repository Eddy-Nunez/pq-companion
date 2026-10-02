# Cross-Client DPS/HPS Sync — Feasibility & Plan

Status: **research only, not scheduled.** Written 2026-10-02 in response to a
community feature request. Priority is the Seekers of Souls website; this is
parked until that is stable.

## The request

Players in the same group/raid share their parses so meters are accurate.
Local EQ logs miss damage from other players: ranged attackers (Rangers) drop
out of melee players' logs due to range clipping, and DoT ticks reliably
register only for the caster. Casters and ranged classes are shortchanged on
every other player's meter.

## Why it matters beyond DPS

Once clients can exchange data, most "my log can't see it" gaps close:

- **HPS** — other players' heals on other targets mostly never log. Likely a
  bigger accuracy win than DPS.
- **Threat Meter** — use real damage from everyone instead of estimates
  (currently dev-gated).
- **Shared debuff timers** — another enchanter's mez/tash, shaman slows,
  everyone's DoTs on the boss.
- **CH chain** — real caster mana and chain position instead of chat inference.
- **Out-of-range trigger broadcasts** — emotes/AEs only nearby players see.
- **Guild-wide shared state** — lockouts, respawns, rolls, loot, attendance,
  PoP flags; would also feed the Seekers site.

## Core design

**Each client publishes only its own outgoing damage and healing** (the lines
its own log records reliably: its melee, DoT ticks, arrows, heals). Receiving
clients replace their partial observed view of that player with that player's
self-report. This avoids most double-counting and keeps the protocol small.

Components:

1. **Relay** — Cloudflare Worker + one Durable Object per room
   (raid/group), hibernatable WebSockets. Ephemeral: forwards live data,
   stores nothing. Small amount of code (days).
2. **Room membership** — room codes first; later tie to Discord OAuth (shared
   with the Seekers site). Roster can be validated against
   `/outputfile raidlist` / raid-composition decode.
3. **Encounter alignment (the hard part)**
   - PC clocks drift and log timestamps are 1-second resolution; each client
     reports its clock offset.
   - Agreeing on which fight/mob ("a gnoll" x5 is ambiguous); Zeal spawn IDs
     help where available. See the NPC variant disambiguation work.
   - Fight boundaries from the existing kill/timeout logic in `combat`.
4. **Partial adoption** — non-PQC players fall back to locally observed
   numbers. UI must distinguish *reported* vs *observed*.
5. **Trust** — a modified client can inflate numbers. Treat as "reported",
   never "verified".
6. **App plumbing** — opt-in settings toggle, WebSocket client in the Go
   backend, merge layer in `internal/combat`, UI badge on meter rows.

Effort: relay is days; merge/alignment + UI is multi-week with a long tail of
edge cases. Largest networked feature the app would have.

## Privacy / positioning

PQC is built to be self-contained: no gameplay or personal data leaves the
machine unless the user sets it up. (It does make outbound requests for update
checks, Zeal/eqw version checks, the QuarmPatcher manifest, and user-configured
Discord webhooks.) Sync would be the first feature that sends gameplay data to
other people. Must be **strictly opt-in**, described as peer data sharing, not
"telemetry" (which implies reporting to the developer).

## Cloudflare cost model ($5 Workers Paid)

Durable Objects pricing (verified against Cloudflare docs 2026-10-02):
requests 1M included then $0.15/M; **incoming WebSocket messages bill 20:1**
(outbound fan-out is not billed as requests); duration 400k GB-s included then
$12.50/M GB-s at 128 MB per active object; hibernated objects cost nothing.

Key insight: **cost scales with the number of active rooms, not players.**
Messages are cheap; compute duration is the driver.

Assumptions (estimates, not measured): 1,200 players connected 3 h/night,
30 nights/month, one batch per 2 s during combat only.

| Scenario | Billed requests | Compute | Approx. overage/mo |
|---|---|---|---|
| 20 players, 1 raid room | ~2.7k/night | ~1.35k GB-s/night | ~$0 (fits included) |
| 60 raid rooms of ~20, active half the time | ~4.9M/mo | ~1.2M GB-s | ~$10 |
| 60 raid rooms, always active | ~9.7M/mo | ~2.4M GB-s | ~$27 |
| 200 six-person group rooms, always active | ~9.7M/mo | ~8.1M GB-s | ~$95 |

Overage is on top of the $5 base. Realistic adoption is likely 5-20% of the
server, so these are worst cases.

Mitigations:
- Batch every 5-10 s instead of 2 s (halves requests; compute only drops if
  rooms actually reach hibernation, which needs ~10 s of idle).
- Pack several rooms into one Durable Object (up to ~10x compute reduction).
  A single object tops out around ~1,000 req/s, so cap rooms per object.
- Per-room caps and rate limits.
- Keep it a **separate Worker** from the Seekers site so usage is attributable.
- Decide up front who pays if it grows (Ko-fi, caps, or opt-in limits).

Open item: DO CPU-time billing for broadcast fan-out was not verified; check
current Workers Paid CPU pricing before committing to a number.

## Operational concerns at server scale

Cost is the smaller concern. The bigger ones:

- **Abuse/spam** — open rooms invite fake data, harassment, flooding. Needs
  auth (Discord OAuth), rate limiting, room controls.
- **Reliability expectations** — once raids depend on it, outages land on the
  maintainer on raid night.
- **Support load** — "my parse doesn't match" reports from clock drift and
  same-name mobs.
- **Privacy/policy** — retention (target: none), privacy policy, consent UX.
- **Protocol versioning** — old and new clients must interoperate, so version
  the wire format from day one.

## If we ever do it: suggested phasing

1. **Phase 0** — decide hosting/cost ownership; write privacy language.
2. **Phase 1 (MVP)** — opt-in room codes, relay Worker + DO, each client
   shares only own damage + healing totals per encounter, merged view marked
   "reported". Small groups first.
3. **Phase 2** — encounter alignment hardening (clock offset, spawn IDs),
   raid-scale rooms, Discord auth.
4. **Phase 3** — extend the channel: shared debuff timers, CH chain, Threat
   Meter inputs, guild state / Seekers site integration.

Related: `docs/guild-website-feasibility.md` (Cloudflare + Discord OAuth
plan), `docs/eqlogparser-dps-architecture.md`.

## Community reply

The request was filed on the Discord. Reply draft: open to investigating,
clear that it is a large, long-term, hosted-service commitment that changes the
app's local-only privacy model, not the current priority, would start small
(opt-in group/raid rooms sharing own damage + healing).
