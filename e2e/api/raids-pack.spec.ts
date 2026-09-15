// Integration tests for the raid composition PACK export/import surface
// (backend/internal/api/raidpacks.go). Runs against a live backend
// (PQ_BASE_URL) and the REAL user.db — every created object is namespaced
// `e2e-pack-<run>` and cleaned up in afterAll; nothing here touches seed
// data (the only pre-existing id used read-only is the seeded `aow`).
//
// Covered:
//   - Export-all envelope (kind/version/exported_at) + includes the seed
//   - Per-encounter export (children carried) + 404 for a missing id
//   - Preview structural rejections (kind/version/empty/invalid JSON)
//   - Preview annotations: exists flag, unknown-role + duplicate-comp
//     errors, zero-zone_id warning — and preview persists nothing
//   - Commit: new encounter saved with source=import + children persisted;
//     hand-built invalid payload → per-item failed, no row written;
//     duplicate-comp commit → failed + no partial write
//   - Conflict semantics: default skip leaves the local record untouched;
//     overwrite applies pack content but preserves local provenance
//     (source) — provenance here is a distinctive CRUD-stamped source so
//     preservation is actually observable
//   - Full round-trip: export → delete → re-import → identical content
import { expect, test } from '@playwright/test'

const RUN = Date.now().toString(36)
const LOCAL = `e2e-pack-${RUN}-local` // CRUD-created, provenance checks
const IMP = `e2e-pack-${RUN}-imp` // import-created, children + round-trip
const BAD = `e2e-pack-${RUN}-bad` // hand-built invalid commit target
const DUP = `e2e-pack-${RUN}-dup` // duplicate-comp rollback target

const ALL_IDS = [LOCAL, IMP, BAD, DUP]

function packEncounter(
  id: string,
  over: Partial<{
    name: string
    zone_id: number
    comps: Array<{ role: string; sub_role?: string; min: number; rec: number }>
    reqs: string[]
    strategy: Record<string, string>
    status: string
    source: string
  }> = {},
) {
  return {
    id,
    name: over.name ?? 'Pack Round Trip',
    zone: 'Kael Drakkel',
    zone_id: over.zone_id ?? 113,
    status: over.status ?? 'active',
    source: over.source,
    comps: over.comps ?? [
      { role: 'tank', sub_role: 'defensive', min: 1, rec: 2 },
      { role: 'damage', min: 5, rec: 6 },
    ],
    reqs: over.reqs ?? ['pack req one', 'pack req two'],
    strategy: over.strategy ?? { notes: 'pack strategy', pulling: 'pack pull' },
  }
}

function pack(...encounters: unknown[]) {
  return {
    kind: 'pq-companion.raidcomp-pack',
    version: 1,
    pack_name: 'e2e pack',
    exported_at: 0,
    encounters,
  }
}

test.describe.serial('raid pack export/import', () => {
  test.beforeAll(async ({ request }) => {
    // Pre-clean leftovers from a failed earlier run within THIS process's
    // id space is impossible (RUN differs), but keep deletes best-effort so
    // a crash mid-run doesn't wedge the next run's assertions.
    for (const id of ALL_IDS) {
      await request.delete(`/api/raids/encounters/${id}`)
    }
  })

  test.afterAll(async ({ request }) => {
    for (const id of ALL_IDS) {
      await request.delete(`/api/raids/encounters/${id}`)
    }
  })

  test('export-all returns a valid pack envelope including the seed', async ({
    request,
  }) => {
    const res = await request.get('/api/raids/export')
    expect(res.status()).toBe(200)
    const pack = await res.json()
    expect(pack.kind).toBe('pq-companion.raidcomp-pack')
    expect(pack.version).toBe(1)
    expect(pack.exported_at).toBeGreaterThan(0)
    expect(Array.isArray(pack.encounters)).toBe(true)
    expect(pack.encounters.length).toBeGreaterThan(0)
    const ids = pack.encounters.map((e: { id: string }) => e.id)
    expect(ids).toContain('aow') // seeded knowledge base ships in export-all
    for (const e of pack.encounters) {
      expect(typeof e.id).toBe('string')
      expect(typeof e.name).toBe('string')
      expect(Array.isArray(e.comps)).toBe(true)
    }
  })

  test('per-encounter export carries children; missing id 404s', async ({
    request,
  }) => {
    const res = await request.get('/api/raids/encounters/aow/export')
    expect(res.status()).toBe(200)
    const pack = await res.json()
    expect(pack.encounters.length).toBe(1)
    expect(pack.encounters[0].id).toBe('aow')
    expect(pack.encounters[0].comps.length).toBeGreaterThan(0)

    const missing = await request.get('/api/raids/encounters/e2e-nope/export')
    expect(missing.status()).toBe(404)
  })

  test('preview rejects structurally invalid packs without persisting', async ({
    request,
  }) => {
    const cases: Array<{
      name: string
      data: unknown
      err: string
    }> = [
      {
        data: { kind: 'other', version: 1, encounters: [] },
        err: 'not a raid composition pack',
      },
      {
        data: { kind: 'pq-companion.raidcomp-pack', version: 99, encounters: [] },
        err: 'unsupported pack version',
      },
      {
        data: { kind: 'pq-companion.raidcomp-pack', version: 1, encounters: [] },
        err: 'no encounters',
      },
    ]
    for (const c of cases) {
      const res = await request.post('/api/raids/import/preview', { data: c.data })
      expect(res.status()).toBe(400)
      expect(await res.text()).toContain(c.err)
    }
    const junk = await request.post('/api/raids/import/preview', {
      data: '{not json',
      headers: { 'content-type': 'application/json' },
    })
    expect(junk.status()).toBe(400)
  })

  test('preview annotates conflicts, bad roles, dup comps, missing zone', async ({
    request,
  }) => {
    const res = await request.post('/api/raids/import/preview', {
      data: pack(
        packEncounter(IMP, { zone_id: 0 }), // fresh id, no zone -> warning
        packEncounter('aow'), // collides with the seed -> exists
        packEncounter(`${IMP}-bad`, {
          comps: [
            { role: 'tank', sub_role: 'defensive', min: 1, rec: 2 },
            { role: 'not_a_role', min: 1, rec: 1 },
          ],
        }), // unknown role -> error
        packEncounter(`${IMP}-dup`, {
          comps: [
            { role: 'tank', sub_role: 'defensive', min: 1, rec: 2 },
            { role: 'tank', sub_role: 'defensive', min: 3, rec: 4 },
          ],
        }), // duplicate comp path -> error
      ),
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.pack_name).toBe('e2e pack')
    const byId: Record<
      string,
      { exists: boolean; errors?: string[]; warnings?: string[] }
    > = {}
    for (const item of body.encounters) byId[item.encounter.id] = item

    expect(byId[IMP].exists).toBe(false)
    expect((byId[IMP].warnings ?? []).join(' ')).toContain('zone_id')

    expect(byId.aow.exists).toBe(true)

    expect((byId[`${IMP}-bad`].errors ?? []).join(' ')).toContain(
      'not in taxonomy',
    )
    expect((byId[`${IMP}-dup`].errors ?? []).join(' ')).toContain(
      'duplicate comp row',
    )

    // Preview must not have persisted anything.
    const probe = await request.get(`/api/raids/encounters/${IMP}`)
    expect(probe.status()).toBe(404)
  })

  test('commit installs a new encounter with source=import', async ({
    request,
  }) => {
    const res = await request.post('/api/raids/import/commit', {
      data: {
        pack_name: 'e2e pack',
        encounters: [{ encounter: packEncounter(IMP), overwrite: false }],
      },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.saved).toEqual([IMP])
    expect(body.skipped).toEqual([])
    expect(body.failed ?? {}).toEqual({})

    const got = await (await request.get(`/api/raids/encounters/${IMP}`)).json()
    expect(got.source).toBe('import')
    expect(got.comps.length).toBe(2)
    expect(got.reqs).toEqual(['pack req one', 'pack req two'])
    expect(got.strategy.notes).toBe('pack strategy')
  })

  test('hand-built invalid commit fails per-item without writing', async ({
    request,
  }) => {
    const res = await request.post('/api/raids/import/commit', {
      data: {
        encounters: [
          {
            encounter: packEncounter(BAD, {
              comps: [
                { role: 'tank', sub_role: 'defensive', min: 1, rec: 2 },
                { role: 'not_a_role', min: 1, rec: 1 },
              ],
            }),
            overwrite: false,
          },
        ],
      },
    })
    expect(res.status()).toBe(200) // per-item outcome, not a 4xx
    const body = await res.json()
    expect(Object.keys(body.failed ?? {})).toEqual([BAD])
    expect(body.failed[BAD]).toContain('not in taxonomy')
    expect(body.saved).toEqual([])

    // The failed item must not exist — no partial write.
    const probe = await request.get(`/api/raids/encounters/${BAD}`)
    expect(probe.status()).toBe(404)
  })

  test('duplicate comp rows fail per-item with no partial write', async ({
    request,
  }) => {
    const res = await request.post('/api/raids/import/commit', {
      data: {
        encounters: [
          {
            encounter: packEncounter(DUP, {
              comps: [
                { role: 'tank', sub_role: 'defensive', min: 1, rec: 2 },
                { role: 'tank', sub_role: 'defensive', min: 3, rec: 4 },
              ],
            }),
            overwrite: false,
          },
        ],
      },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Object.keys(body.failed ?? {})).toEqual([DUP])
    const probe = await request.get(`/api/raids/encounters/${DUP}`)
    expect(probe.status()).toBe(404)
  })

  test('skip is the default conflict choice; overwrite preserves provenance', async ({
    request,
  }) => {
    // A CRUD-created encounter with a distinctive source stamp.
    const made = await request.post('/api/raids/encounters', {
      data: packEncounter(LOCAL, { source: 'e2e-local-provenance' }),
    })
    expect(made.status()).toBe(201)

    // Re-committing the same id with the default (overwrite=false) is skipped.
    const skip = await request.post('/api/raids/import/commit', {
      data: {
        encounters: [
          {
            encounter: packEncounter(LOCAL, { name: 'Pack Renamed' }),
            overwrite: false,
          },
        ],
      },
    })
    expect(skip.status()).toBe(200)
    const skipBody = await skip.json()
    expect(skipBody.skipped).toEqual([LOCAL])
    expect(skipBody.saved).toEqual([])

    // Local record untouched by the skip.
    let got = await (await request.get(`/api/raids/encounters/${LOCAL}`)).json()
    expect(got.name).toBe('Pack Round Trip')
    expect(got.source).toBe('e2e-local-provenance')
    expect(got.comps.length).toBe(2)

    // Overwrite applies the pack content but keeps the local source stamp,
    // and children are replaced wholesale (1 comp, 1 req, 1 strategy section).
    const ovr = await request.post('/api/raids/import/commit', {
      data: {
        encounters: [
          {
            encounter: packEncounter(LOCAL, {
              name: 'Pack Renamed',
              comps: [{ role: 'damage', min: 9, rec: 10 }],
              reqs: ['only req'],
              strategy: { notes: 'only strategy' },
            }),
            overwrite: true,
          },
        ],
      },
    })
    expect(ovr.status()).toBe(200)
    const ovrBody = await ovr.json()
    expect(ovrBody.saved).toEqual([LOCAL])
    expect(ovrBody.skipped).toEqual([])

    got = await (await request.get(`/api/raids/encounters/${LOCAL}`)).json()
    expect(got.name).toBe('Pack Renamed')
    expect(got.source).toBe('e2e-local-provenance')
    expect(got.comps.length).toBe(1)
    expect(got.comps[0].role).toBe('damage')
    expect(got.reqs).toEqual(['only req'])
    expect(got.strategy).toEqual({ notes: 'only strategy' })
  })

  test('round-trip: export -> delete -> re-import restores content', async ({
    request,
  }) => {
    const exportRes = await request.get(`/api/raids/encounters/${IMP}/export`)
    expect(exportRes.status()).toBe(200)
    const exported = await exportRes.json()
    expect(exported.encounters.length).toBe(1)

    const del = await request.delete(`/api/raids/encounters/${IMP}`)
    expect(del.status()).toBe(204)
    expect((await request.get(`/api/raids/encounters/${IMP}`)).status()).toBe(
      404,
    )

    // Re-import the exact pack we exported (as the wizard would relay it).
    const commit = await request.post('/api/raids/import/commit', {
      data: {
        pack_name: exported.pack_name,
        encounters: exported.encounters.map((e: unknown) => ({
          encounter: e,
          overwrite: false,
        })),
      },
    })
    expect(commit.status()).toBe(200)
    const commitBody = await commit.json()
    expect(commitBody.saved).toEqual([IMP])

    const restored = await (
      await request.get(`/api/raids/encounters/${IMP}`)
    ).json()
    expect(restored.name).toBe('Pack Round Trip')
    expect(restored.comps.length).toBe(2)
    expect(restored.reqs).toEqual(['pack req one', 'pack req two'])
    expect(restored.strategy.notes).toBe('pack strategy')
    // Re-created by import → source stamp re-applied.
    expect(restored.source).toBe('import')
  })

  test('commit guard rails: empty selection and unknown fields', async ({
    request,
  }) => {
    const empty = await request.post('/api/raids/import/commit', {
      data: { encounters: [] },
    })
    expect(empty.status()).toBe(400)

    const unknown = await request.post('/api/raids/import/commit', {
      data: { encounters: [], extra: 1 },
    })
    expect(unknown.status()).toBe(400)
  })

  test.afterAll(async ({ request }) => {
    // Verify cleanup actually left nothing behind (belt: the deletes above;
    // braces: re-delete is idempotent 204).
    const list = await (await request.get('/api/raids/encounters')).json()
    const leftovers = (list.encounters as Array<{ id: string }>).filter((e) =>
      e.id.startsWith(`e2e-pack-${RUN}`),
    )
    expect(leftovers).toEqual([])
  })
})
