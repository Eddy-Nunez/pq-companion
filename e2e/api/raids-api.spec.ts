// Integration tests for the raid REST surface, against a running Go backend
// (PQ_BASE_URL, default the dev handshake port 17654). The raidcomp store
// self-seeds the role TAXONOMY on first open; encounters are NOT auto-seeded
// (upstream a12bc7a5), so encounter-dependent tests create their own fixtures
// and clean up.
import { expect, test } from '@playwright/test'

test.describe('GET /api/raids/taxonomy', () => {
  test('returns the seeded role taxonomy', async ({ request }) => {
    const res = await request.get('/api/raids/taxonomy')
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Object.keys(body.roles).length).toBeGreaterThan(0)
    expect(Array.isArray(body.role_order)).toBe(true)
    for (const role of Object.values<{
      label: string
      classes?: string[]
      sub_roles?: Record<string, { label: string; classes: string[] }>
    }>(body.roles)) {
      expect(typeof role.label).toBe('string')
      // A role is either flat (own class list) or a parent of sub-roles;
      // parents omit classes. Leaves always carry a REAL classes array —
      // possibly empty: import-provisioned stub roles legitimately have no
      // class mappings until the user assigns them (never null though).
      if (role.classes !== undefined) {
        expect(Array.isArray(role.classes)).toBe(true)
      } else {
        expect(Object.keys(role.sub_roles ?? {}).length).toBeGreaterThan(0)
        for (const sub of Object.values(role.sub_roles ?? {})) {
          expect(Array.isArray(sub.classes)).toBe(true)
        }
      }
    }
  })
})

test.describe('GET /api/raids/roster', () => {
  test('reports Zeal connectivity and roster state', async ({ request }) => {
    const res = await request.get('/api/raids/roster')
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(typeof body.zeal_connected).toBe('boolean')
    expect(typeof body.in_raid).toBe('boolean')
    expect(Array.isArray(body.members)).toBe(true)
    // A live roster's members carry the fields the checker consumes.
    for (const m of body.members) {
      expect(typeof m.name).toBe('string')
   expect(typeof m.code === 'string' || m.code === undefined).toBe(true)
    }
  })
  test('connected pipe always reports the live zone (live-zone overlay)', async ({
    request,
  }) => {
    const res = await request.get('/api/raids/roster')
    expect(res.status()).toBe(200)
    const body = await res.json()
    if (!body.zeal_connected) return // no EQ/Zeal running (CI) — nothing to pin
    // The snapshot's own zone is stamped when a MsgRaid arrives and goes
    // stale when the raid zones without a membership change, so the endpoint
    // overlays the CURRENT pipe zone on read (liveZone atomic publish from
    // MsgPlayer). A connected pipe must therefore always report a resolvable
    // zone for encounter context, regardless of MsgRaid cadence.
    expect(body.zone_id).toBeGreaterThan(0)
    expect(typeof body.zone).toBe('string')
    expect(body.zone.length).toBeGreaterThan(0)
    // Decode contract (d6d385fb): Zeal's raid wire format carries class as a
    // display NAME ("Wizard") and level as a STRING ("60"); the decoder
    // normalizes both, so members expose numeric level/class plus the
    // taxonomy code resolved at capture time (code omitted when unknown).
    for (const m of body.members) {
      expect(typeof m.level).toBe('number')
      expect(typeof m.class).toBe('number')
      expect(typeof m.code === 'string' || m.code === undefined).toBe(true)
    }
  })
})

test.describe('GET /api/raids/encounters', () => {
  const RUN = Date.now().toString(36)
  const ENC = `e2e-api-enc-${RUN}`

  test.afterAll(async ({ request }) => {
    await request.delete(`/api/raids/encounters/${ENC}`)
  })

  test('lists created encounters (knowledge base starts empty)', async ({
    request,
  }) => {
    const mk = await request.post('/api/raids/encounters', {
      data: {
        id: ENC,
        name: 'API Fixture Boss',
        zone: 'Kael Drakkel',
        status: 'active',
        comps: [{ role: 'damage', min: 2, rec: 3 }],
      },
    })
    expect(mk.status()).toBe(201)

    const res = await request.get('/api/raids/encounters')
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body.encounters)).toBe(true)
    const fixture = body.encounters.find((e: { id: string }) => e.id === ENC)
    expect(fixture, 'created fixture appears in the list').toBeTruthy()
  })
})

test.describe('POST /api/raids/check', () => {
  const RUN = Date.now().toString(36)
  const ENC = `e2e-api-check-${RUN}`

  test.afterAll(async ({ request }) => {
    await request.delete(`/api/raids/encounters/${ENC}`)
  })

  test('produces a structured MIN/REC report for a tiny roster', async ({
    request,
  }) => {
    const mk = await request.post('/api/raids/encounters', {
      data: {
        id: ENC,
        name: 'Check Fixture Boss',
        zone: 'Kael Drakkel',
        status: 'active',
        comps: [
          { role: 'tank', sub_role: 'defensive', min: 1, rec: 2 },
          { role: 'damage', min: 5, rec: 6 },
        ],
      },
    })
    expect(mk.status()).toBe(201)

    const res = await request.post('/api/raids/check', {
      data: {
        encounter_id: ENC,
        roster: [
          { name: 'Thud', class: 'war' },
          { name: 'Anden', class: 'cleric' },
        ],
      },
    })
    expect(res.status()).toBe(200)
    const report = await res.json()
    expect(report.encounter_id).toBe(ENC)
    expect(report.roster_total).toBe(2)
    expect(report.roster_mapped).toBe(2)
    expect(Array.isArray(report.min)).toBe(true)
    expect(Array.isArray(report.rec)).toBe(true)
    expect(typeof report.summary.min.gap_rows).toBe('number')
  })

  test('rejects an unknown encounter', async ({ request }) => {
    const res = await request.post('/api/raids/check', {
      data: { encounter_id: 'no-such-encounter', roster: [] },
    })
    expect(res.status()).toBe(404)
  })

  test('rejects a missing encounter_id', async ({ request }) => {
    const res = await request.post('/api/raids/check', { data: {} })
    expect(res.status()).toBe(400)
  })
})
