// Integration tests for the group-composition shapes API (stages 1+2 of
// docs/raid-group-compositions-plan.md). Runs against a live backend
// (PQ_BASE_URL) — the REAL user.db — so the probe encounter is namespaced
// and cleaned up (pre-clean beforeAll + best-effort afterAll).
//
// Covered:
//   - shapes persist through the encounter API (save → load round trip)
//   - split rejects unknown ids and bad distribution
//   - single-raid proposals weave shapes too (cohort-only lifted 2026-10-05)
//   - replicate: every raid fields the shape; group cards carry shape_id
//   - distribute: shape i applies to raid i
import { expect, test } from '@playwright/test'

const RUN = Date.now().toString(36)
const ENC = `e2e-enc-shapes-${RUN}`

const ENC_BODY = {
  id: ENC,
  name: 'E2E Split Shapes',
  zone: 'Kael Drakkel',
  zone_id: 113,
  status: 'active',
  comps: [
    { role: 'tank', sub_role: 'defensive', min: 1, rec: 1 },
    { role: 'healer', sub_role: 'ch_cleric', min: 2, rec: 2 },
    { role: 'damage', min: 3, rec: 3 },
  ],
  shapes: [
    {
      shape_id: 'e2e-healstack',
      rows: [
        { role: 'healer', sub_role: 'ch_cleric', count: 2 },
        { role: 'tank', sub_role: 'defensive', count: 1 },
      ],
    },
    { shape_id: 'e2e-caststack', rows: [{ role: 'damage', count: 2 }] },
  ],
}

const ROSTER = [
  { name: 'TankA', class: 'war' },
  { name: 'TankB', class: 'war' },
  { name: 'ClericA', class: 'clr' },
  { name: 'ClericB', class: 'clr' },
  { name: 'ClericC', class: 'clr' },
  { name: 'ClericD', class: 'clr' },
  { name: 'Rogue01', class: 'rog' },
  { name: 'Rogue02', class: 'rog' },
  { name: 'Rogue03', class: 'rog' },
  { name: 'Rogue04', class: 'rog' },
  { name: 'Rogue05', class: 'rog' },
  { name: 'Rogue06', class: 'rog' },
]

function splitBody(over: Record<string, unknown>): Record<string, unknown> {
  return { encounter_id: ENC, preference: 'trinity', cohorts: 2, roster: ROSTER, ...over }
}

test.describe.serial('POST /api/raids/split — group-composition shapes', () => {
  test.beforeAll(async ({ request }) => {
    // Pre-clean leftovers from a failed earlier run (ignore errors).
    await request.delete(`/api/raids/encounters/${ENC}`)
    const res = await request.post('/api/raids/encounters', { data: ENC_BODY })
    expect(res.status()).toBe(201)
  })

  test.afterAll(async ({ request }) => {
    // Best-effort cleanup regardless of test outcomes.
    await request.delete(`/api/raids/encounters/${ENC}`)
  })

  test('shapes persist through the encounter save/load round trip', async ({ request }) => {
    const res = await request.get(`/api/raids/encounters/${ENC}`)
    expect(res.status()).toBe(200)
    const enc = await res.json()
    expect(enc.shapes).toHaveLength(2)
    expect(enc.shapes.map((s: { shape_id: string }) => s.shape_id)).toEqual([
      'e2e-healstack',
      'e2e-caststack',
    ])
    expect(enc.shapes[0].rows).toEqual([
      { role: 'healer', sub_role: 'ch_cleric', count: 2 },
      { role: 'tank', sub_role: 'defensive', count: 1 },
    ])
  })

  test('single-raid proposals weave shapes (cohort-only lifted)', async ({ request }) => {
    const res = await request.post('/api/raids/split', {
      data: splitBody({ cohorts: 1, shapes: ['e2e-healstack'] }),
    })
    expect(res.status()).toBe(200)
    const rep = await res.json()
    const g1 = rep.groups[0]
    expect(g1.shape_id).toBe('e2e-healstack')
    expect(g1.slots).toHaveLength(3)
    const cleric = rep.min.find((c: { path: string }) => c.path === 'healer.ch_cleric')
    expect(cleric.placed).toBeGreaterThanOrEqual(2)
  })

  test('rejects an unknown shape id (400)', async ({ request }) => {
    const res = await request.post('/api/raids/split', {
      data: splitBody({ shapes: ['e2e-healstack', 'e2e-nope'] }),
    })
    expect(res.status()).toBe(400)
    expect((await res.json()).error).toContain('unknown shape')
  })

  test('rejects a bad shape_distribution (400)', async ({ request }) => {
    const res = await request.post('/api/raids/split', {
      data: splitBody({ shapes: ['e2e-healstack'], shape_distribution: 'zigzag' }),
    })
    expect(res.status()).toBe(400)
    expect((await res.json()).error).toContain('shape_distribution')
  })

  test('replicate fields the shape in every raid with shape_id on the group card', async ({ request }) => {
    const res = await request.post('/api/raids/split', {
      data: splitBody({ shapes: ['e2e-healstack'] }),
    })
    expect(res.status()).toBe(200)
    const rep = await res.json()
    expect(rep.cohorts).toHaveLength(2)
    for (const raid of rep.cohorts) {
      const g1 = raid.groups[0]
      expect(g1.shape_id).toBe('e2e-healstack')
      // 2 CH clerics + 1 defensive tank consumed by the shape; open seats
      // stay open (the trinity pass skips shape groups).
      expect(g1.slots).toHaveLength(3)
      const paths = g1.slots.map((s: { path: string }) => s.path).sort()
      expect(paths).toEqual(['healer.ch_cleric', 'healer.ch_cleric', 'tank.defensive'])
      // Shape seats satisfy the matching MIN needs.
      const cleric = raid.min.find((c: { path: string }) => c.path === 'healer.ch_cleric')
      expect(cleric.placed).toBe(2)
      const tank = raid.min.find((c: { path: string }) => c.path === 'tank.defensive')
      expect(tank.placed).toBe(1)
      // Everyone seats, and the trinity group holds the 3 rogues.
      expect(raid.groups).toHaveLength(2)
      expect(raid.groups[1].slots).toHaveLength(3)
    }
  })

  test('distribute applies shape i to raid i', async ({ request }) => {
    const res = await request.post('/api/raids/split', {
      data: splitBody({ shapes: ['e2e-healstack', 'e2e-caststack'], shape_distribution: 'distribute' }),
    })
    expect(res.status()).toBe(200)
    const rep = await res.json()
    expect(rep.cohorts[0].groups[0].shape_id).toBe('e2e-healstack')
    expect(rep.cohorts[1].groups[0].shape_id).toBe('e2e-caststack')
    // No shape bled into the other raid's groups.
    for (const g of rep.cohorts[0].groups) expect(g.shape_id === 'e2e-caststack').toBe(false)
    for (const g of rep.cohorts[1].groups) expect(g.shape_id === 'e2e-healstack').toBe(false)
  })
})
