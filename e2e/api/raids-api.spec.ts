// Integration tests for the raid REST surface, against a running Go backend
// (PQ_BASE_URL, default the dev handshake port 17654). The raidcomp store
// self-seeds the role taxonomy and the Avatar of War encounter on first
// open, so these hold on a fresh user.db as well as a lived-in one.
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
      // parents omit classes. Leaves always carry a non-empty class list.
      if (role.classes !== undefined) {
        expect(role.classes.length).toBeGreaterThan(0)
      } else {
        expect(Object.keys(role.sub_roles ?? {}).length).toBeGreaterThan(0)
        for (const sub of Object.values(role.sub_roles ?? {})) {
          expect(sub.classes.length).toBeGreaterThan(0)
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
})

test.describe('GET /api/raids/encounters', () => {
  test('lists encounters including the seeded AoW', async ({ request }) => {
    const res = await request.get('/api/raids/encounters')
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body.encounters)).toBe(true)
    const aow = body.encounters.find((e: { id: string }) => e.id === 'aow')
    expect(aow, 'seeded Avatar of War encounter').toBeTruthy()
  })
})

test.describe('POST /api/raids/check', () => {
  test('produces a structured MIN/REC report for a tiny roster', async ({
    request,
  }) => {
    const res = await request.post('/api/raids/check', {
      data: {
        encounter_id: 'aow',
        roster: [
          { name: 'Thud', class: 'war' },
          { name: 'Anden', class: 'cleric' },
        ],
      },
    })
    expect(res.status()).toBe(200)
    const report = await res.json()
    expect(report.encounter_id).toBe('aow')
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
