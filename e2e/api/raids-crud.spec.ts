// Integration tests for the raid-composition WRITE surface + checker
// semantics. Runs against a live backend (PQ_BASE_URL), which means the REAL
// user.db — so every test object is namespaced `e2e_*` and cleaned up:
// pre-clean beforeAll handles leftovers from a previously failed run.
//
// Covered (features from feat/raid-composition):
//   - Taxonomy roles CRUD (flat + sub-role rows, update-in-place)
//   - Delete-blocked guard: a role still referenced by an encounter comp
//     cannot be deleted (clear 400), and deletes fine once unreferenced
//   - Encounter CRUD with comps/reqs/strategy, validation guard rails
//   - Checker MIN/REC semantics: zero-need rows omitted, min0/rec>0 rows
//     only in REC, gap rollup, candidates on gap rows, unmapped members
import { expect, test } from '@playwright/test'

const RUN = Date.now().toString(36)
const ROLE = 'e2e_probe' // stable so re-runs pre-clean the same row
const ENC = `e2e-enc-${RUN}`

test.describe.serial('raid CRUD + checker semantics', () => {
  test.beforeAll(async ({ request }) => {
    // Pre-clean leftovers from a failed earlier run (ignore errors).
    // DeleteRole is keyed by (role, sub) — sub-rows need their own calls.
    await request.delete(`/api/raids/encounters/${ENC}`)
    await request.delete(`/api/raids/roles?role=${ROLE}`)
    await request.delete(`/api/raids/roles?role=${ROLE}&sub=extra`)
  })

  test.afterAll(async ({ request }) => {
    // Best-effort cleanup regardless of test outcomes.
    await request.delete(`/api/raids/encounters/${ENC}`)
    await request.delete(`/api/raids/roles?role=${ROLE}`)
    await request.delete(`/api/raids/roles?role=${ROLE}&sub=extra`)
  })

  test('role lifecycle: create, appears flat in taxonomy, update', async ({
    request,
  }) => {
    // Create a flat role. Classes must be valid catalog codes.
    const res = await request.post('/api/raids/roles', {
      data: { role: ROLE, label: 'E2E Probe', classes: ['bst'] },
    })
    expect(res.status()).toBe(200)

    const tax = await (await request.get('/api/raids/taxonomy')).json()
    expect(tax.roles[ROLE]).toBeTruthy()
    expect(tax.roles[ROLE].label).toBe('E2E Probe')
    expect(tax.roles[ROLE].classes).toEqual(['bst'])

    // Update in place: new label + widened class list + a sub-role row.
    const upd = await request.post('/api/raids/roles', {
      data: { role: ROLE, label: 'E2E Probe Renamed', classes: ['bst', 'mnk'] },
    })
    expect(upd.status()).toBe(200)
    const sub = await request.post('/api/raids/roles', {
      data: {
        role: ROLE,
        sub_role: 'extra',
        label: 'E2E Probe Extra',
        classes: ['mnk'],
      },
    })
    expect(sub.status()).toBe(200)

    const tax2 = await (await request.get('/api/raids/taxonomy')).json()
    expect(tax2.roles[ROLE].label).toBe('E2E Probe Renamed')
    expect(tax2.roles[ROLE].classes).toEqual(['bst', 'mnk'])
    expect(tax2.roles[ROLE].sub_roles?.extra).toEqual({
      label: 'E2E Probe Extra',
      classes: ['mnk'],
    })
  })

  test('encounter lifecycle: create with comps/reqs/strategy, read back', async ({
    request,
  }) => {
    const payload = {
      id: ENC,
      name: 'E2E Encounter',
      zone: 'Temple of Veeshan',
      zone_id: 0,
      status: 'active',
      trigger: 'Avatar of War engage',
      reqs: ['Ring of Dain Frostreaver IV (ring of dain frostreaver iv)'],
      strategy: { pulling: 'pull to corner', tanking: '3 tanks rotate' },
      notes: 'created by e2e suite',
      comps: [
        { role: ROLE, min: 2, rec: 4 },
        { role: ROLE, sub_role: 'extra', min: 0, rec: 1 },
        { role: 'healer', sub_role: 'ch_cleric', min: 1, rec: 1 },
        { role: 'rgc', min: 0, rec: 0 },
      ],
    }
    const res = await request.post('/api/raids/encounters', { data: payload })
    expect(res.status()).toBe(201)
    const saved = await res.json()
    expect(saved.id).toBe(ENC)
    expect(saved.created_at).toBeGreaterThan(0)
    expect(saved.updated_at).toBeGreaterThan(0)
    expect(saved.comps).toHaveLength(4)

    // Round-trip: children persisted with identity + min/rec per leaf.
    const got = await request.get(`/api/raids/encounters/${ENC}`)
    expect(got.status()).toBe(200)
    const enc = await got.json()
    expect(enc.zone).toBe('Temple of Veeshan')
    expect(enc.reqs).toEqual(payload.reqs)
    expect(enc.strategy).toEqual(payload.strategy)
    const flat = enc.comps.find(
      (c: { role: string; sub_role?: string }) =>
        c.role === ROLE && !c.sub_role,
    )
    expect(flat).toMatchObject({ min: 2, rec: 4 })
  })

  test('role referenced by an encounter comp cannot be deleted', async ({
    request,
  }) => {
    const res = await request.delete(`/api/raids/roles?role=${ROLE}`)
    expect(res.status()).toBe(400)
    const body = await res.json()
    expect(String(body.error)).toMatch(/referenc|comp|encounter/i)
  })

  test('checker MIN/REC semantics on the custom encounter', async ({
    request,
  }) => {
    // Roster: 2 bst (cover ROLE min, short of rec 4), 1 cleric via class
    // NAME (name-mapping is a feature), 1 garbage class (unmappable).
    const res = await request.post('/api/raids/check', {
      data: {
        encounter_id: ENC,
        roster: [
          { name: 'Bst2', class: 'bst' },
          { name: 'Bst1', class: 'bst' },
          { name: 'Healy', class: 'Cleric' },
          { name: 'Roguey', class: 'zzz_not_a_class' },
        ],
      },
    })
    expect(res.status()).toBe(200)
    const rep = await res.json()

    expect(rep.encounter_id).toBe(ENC)
    expect(rep.roster_total).toBe(4)
    expect(rep.roster_mapped).toBe(3) // garbage class excluded

    // MIN: zero-need rows (rgc, ROLE.extra min0) omitted; ROLE need2 have2.
    const minPaths = rep.min.map((r: { path: string }) => r.path)
    expect(minPaths).toContain(ROLE)
    expect(minPaths).not.toContain('rgc')
    expect(rep.min).toHaveLength(2) // e2e_probe + healer.ch_cleric
    const probeMin = rep.min.find((r: { role: string }) => r.role === ROLE)
    expect(probeMin).toMatchObject({ need: 2, have: 2 })
    expect(probeMin.candidates).toBeUndefined() // met rows carry no candidates
    expect(rep.summary.min).toMatchObject({ ok: true, rows: 2, gap_rows: 0 })

    // REC: ROLE need4 have2 (GAP 2, candidates sorted, capped), extra
    // min0/rec1 appears ONLY here, rgc omitted everywhere.
    const recPaths = rep.rec.map((r: { path: string }) => r.path)
    expect(recPaths).toContain(ROLE)
    expect(recPaths).toContain(`${ROLE}.extra`)
    expect(recPaths).not.toContain('rgc')
    expect(rep.min.map((r: { path: string }) => r.path)).not.toContain(
      `${ROLE}.extra`,
    )
    const probeRec = rep.rec.find((r: { role: string }) => r.role === ROLE)
    expect(probeRec).toMatchObject({ need: 4, have: 2 })
    expect(probeRec.candidates).toEqual(['Bst1', 'Bst2'])
    const extraRec = rep.rec.find(
      (r: { path: string }) => r.path === `${ROLE}.extra`,
    )
    expect(extraRec).toMatchObject({ need: 1, have: 0 })
    expect(extraRec.candidates ?? []).toEqual([]) // no mnk on roster

    // Gap rollup: probe gap 2 + extra gap 1 = 3 across 2 rows.
    expect(rep.summary.rec).toMatchObject({
      ok: false,
      gap_rows: 2,
      gap_count: 3,
    })
  })

  test('encounter validation guard rails', async ({ request }) => {
    const base = { name: 'E2E Bad', zone: 'Somewhere', status: 'active' }

    // Invalid status enum.
    let res = await request.post('/api/raids/encounters', {
      data: { ...base, id: `e2e-bad-${RUN}`, status: 'bogus', comps: [] },
    })
    expect(res.status()).toBe(400)

    // Missing zone.
    res = await request.post('/api/raids/encounters', {
      data: { id: `e2e-bad2-${RUN}`, name: 'x', zone: '', status: 'active', comps: [] },
    })
    expect(res.status()).toBe(400)

    // Unknown taxonomy role in comps.
    res = await request.post('/api/raids/encounters', {
      data: {
        ...base,
        id: `e2e-bad3-${RUN}`,
        comps: [{ role: 'e2e_role_that_does_not_exist', min: 1, rec: 1 }],
      },
    })
    expect(res.status()).toBe(400)

    // Unknown fields are rejected outright (strict decoder).
    res = await request.post('/api/raids/check', {
      data: { encounter_id: 'aow', bogus_field: 1 },
    })
    expect(res.status()).toBe(400)
  })

  test('encounter update replaces comps and the checker follows', async ({
    request,
  }) => {
    // Shrink to a single comp row — SaveEncounter replaces children.
    const res = await request.put(`/api/raids/encounters/${ENC}`, {
      data: {
        name: 'E2E Encounter',
        zone: 'Temple of Veeshan',
        zone_id: 0,
        status: 'active',
        comps: [{ role: 'healer', sub_role: 'ch_cleric', min: 1, rec: 1 }],
      },
    })
    expect(res.status()).toBe(200)

    const got = await request.get(`/api/raids/encounters/${ENC}`)
    const enc = await got.json()
    expect(enc.comps).toHaveLength(1)
    expect(enc.comps[0]).toMatchObject({
      role: 'healer',
      sub_role: 'ch_cleric',
      min: 1,
      rec: 1,
    })

    // Checker now assesses exactly one leaf.
    const rep = await (
      await request.post('/api/raids/check', {
        data: { encounter_id: ENC, roster: [{ name: 'Healy', class: 'clr' }] },
      })
    ).json()
    expect(rep.min).toHaveLength(1)
    expect(rep.summary.min.ok).toBe(true)
  })

  test('encounter delete is idempotent (204 both times)', async ({
    request,
  }) => {
    const res = await request.delete(`/api/raids/encounters/${ENC}`)
    expect(res.status()).toBe(204) // NoContent on success
    expect((await request.get(`/api/raids/encounters/${ENC}`)).status()).toBe(404)
    // DeleteEncounter does not check existence — re-delete is a silent 204
    // (idempotent). Asserted as-is so a future 404-on-miss change flips
    // this loudly.
    expect((await request.delete(`/api/raids/encounters/${ENC}`)).status()).toBe(
      204,
    )
  })

  test('role delete succeeds once unreferenced; taxonomy drops it', async ({
    request,
  }) => {
    // DeleteRole is keyed by (role, sub) — remove the sub row first, then
    // the flat row (no cascade: per-row deletes, mirroring the editor UI).
    expect(
      (await request.delete(`/api/raids/roles?role=${ROLE}&sub=extra`)).status(),
    ).toBe(204)
    const res = await request.delete(`/api/raids/roles?role=${ROLE}`)
    expect(res.status()).toBe(204)
    const tax = await (await request.get('/api/raids/taxonomy')).json()
    expect(tax.roles[ROLE]).toBeUndefined()
    expect(tax.role_order).not.toContain(ROLE)
  })
})
