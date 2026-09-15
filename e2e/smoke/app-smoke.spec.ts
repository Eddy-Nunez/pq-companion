// Renderer smoke tests, run in plain Chromium against the vite-served
// frontend (vite.e2e.config.ts). The backend must be up on the dev port —
// without Electron the renderer falls back to http://127.0.0.1:17654
// (frontend/src/services/backendUrl.ts).
//
// The app uses HashRouter (App.tsx), so page URLs are /#/<route>. Sections
// in the sidebar are collapsible; the Raid Composition page lives at
// /#/raids under the "Raids" section.
//
// Note: the app may pop a "what's new" changelog dialog on a fresh user.db;
// these tests assume the dev machine's store, where it is already dismissed.
//
// The Raids sidebar section (and its routes) are gated behind the
// raids_enabled developer flag (upstream gates the feature for live
// validation), so the Raid tests toggle it via the config API and restore
// the prior value afterwards.
import { expect, test } from '@playwright/test'

const apiBase = process.env.PQ_BASE_URL ?? 'http://127.0.0.1:17654'

test.describe.serial('Raid Composition page', () => {
  let raidsWasEnabled = false

  test.beforeAll(async ({ request }) => {
    const cfg = await (await request.get(`${apiBase}/api/config`)).json()
    raidsWasEnabled = Boolean(cfg?.preferences?.raids_enabled)
    if (!raidsWasEnabled) {
      cfg.preferences.raids_enabled = true
      await request.put(`${apiBase}/api/config`, { data: cfg })
    }
  })

  test.afterAll(async ({ request }) => {
    if (!raidsWasEnabled) {
      const cfg = await (await request.get(`${apiBase}/api/config`)).json()
      cfg.preferences.raids_enabled = false
      await request.put(`${apiBase}/api/config`, { data: cfg })
    }
  })

  test.beforeEach(async ({ page }) => {
    // Upstream moved the check page to /raids/check (/#/raids is now the
    // Raid Summary dashboard).
    await page.goto('/#/raids/check', { waitUntil: 'domcontentloaded' })
    await expect(
      page.getByRole('heading', { name: 'Raid Composition Check' }),
    ).toBeVisible()
  })

  test('shows one of the three roster-source states', async ({ page }) => {
    // Zeal disconnected / connected-not-in-raid / live roster — the card
    // always renders exactly one of these messages.
    await expect(
      page.getByText(
        /problem with the Zeal connection|not in a raid|Live roster \(Zeal pipe\)/,
      ),
    ).toBeVisible()
  })

  test('Refresh is always enabled (regression: 05a9598f)', async ({ page }) => {
    const refresh = page.getByRole('button', { name: 'Refresh' })
    await expect(refresh).toBeVisible()
    await expect(refresh).toBeEnabled()
    // Clicking it must not throw or disable the page — the spinner state is
    // transient, so just assert it is still enabled afterwards.
    await refresh.click()
    await expect(refresh).toBeEnabled()
  })

  test('encounter dropdown is present', async ({ page }) => {
    // The encounter select sits in the header row next to the Refresh
    // button. Wait for encounters to load (or the empty "No encounters"
    // placeholder on a fresh store).
    await expect(page.locator('select').first()).toBeVisible()
  })

  test('manual roster form adds a member row', async ({ page }) => {
    await page.getByRole('button', { name: 'Add member' }).click()
    await expect(page.getByPlaceholder('Member name').first()).toHaveValue(
      'Tank',
    )
    await page
      .getByPlaceholder(/class code or name/i)
      .first()
      .fill('cleric')
  })

  test('manual pick survives Refresh and the report re-runs (regression: live-raids feedback)', async ({
    page,
  }) => {
    const select = page.locator('select').first()
    await expect(select).toBeVisible()
    // Need at least two encounters to prove the pick isn't reset.
    test.skip((await select.locator('option').count()) < 2, 'needs ≥2 encounters')

    // Zone-based auto-detection was removed: multiple encounters can share a
    // zone (Kael has two), so detection kept clobbering manual picks. Pick
    // the SECOND encounter (not the store-order first), run a check, then
    // hit Refresh — the pick must hold AND the report must re-render from
    // the re-run check, not clear back to the placeholder.
    await select.selectOption({ index: 1 })
    const picked = await select.inputValue()
    // Upstream a00611e1: picking an encounter auto-runs the check (no button
    // click). Upstream 907dbf91 merged MIN/REC into one report table; the
    // report header's "members classed" line renders whenever a report exists.
    await expect(page.getByText(/members classed/).first()).toBeVisible()

    await page.getByRole('button', { name: 'Refresh' }).click()
    await expect(page.getByRole('button', { name: 'Refresh' })).toBeEnabled()
    await expect(select).toHaveValue(picked)
    await expect(page.getByText(/members classed/).first()).toBeVisible()
  })
})

test.describe.serial('Raid Editor page', () => {
  let raidsWasEnabled = false
  const RUN = Date.now().toString(36)
  const ENC = `e2e-smoke-enc-${RUN}`

  test.beforeAll(async ({ request }) => {
    const cfg = await (await request.get(`${apiBase}/api/config`)).json()
    raidsWasEnabled = Boolean(cfg?.preferences?.raids_enabled)
    if (!raidsWasEnabled) {
      cfg.preferences.raids_enabled = true
      await request.put(`${apiBase}/api/config`, { data: cfg })
    }
    // Stores no longer auto-seed encounters (upstream a12bc7a5) — create a
    // fixture so the editor has something to list, whatever the user.db state.
    const mk = await request.post(`${apiBase}/api/raids/encounters`, {
      data: {
        id: ENC,
        name: 'Smoke Fixture Boss',
        zone: 'Kael Drakkel',
        status: 'active',
        comps: [{ role: 'damage', min: 5, rec: 8 }],
      },
    })
    expect(mk.status()).toBe(201)
  })

  test.afterAll(async ({ request }) => {
    await request.delete(`${apiBase}/api/raids/encounters/${ENC}`)
    if (!raidsWasEnabled) {
      const cfg = await (await request.get(`${apiBase}/api/config`)).json()
      cfg.preferences.raids_enabled = false
      await request.put(`${apiBase}/api/config`, { data: cfg })
    }
  })

  test('lists encounters and offers the taxonomy editor', async ({ page }) => {
    await page.goto('/#/raids/editor', { waitUntil: 'domcontentloaded' })
    // The fixture encounter must appear in the encounter list.
    await expect(page.getByText('Smoke Fixture Boss').first()).toBeVisible()
    // The taxonomy editor section renders (seeded labels, not raw ids).
    await expect(
      page.getByText(/Role Taxonomy|Remove Greater Curse|Tank/i).first(),
    ).toBeVisible()
  })

  test('taxonomy editor renders a class-less import stub without crashing', async ({
    page,
    request,
  }) => {
    // Regression: a provisioned stub role marshaled classes as JSON null and
    // TaxonomyEditor crashed the whole editor page on render. Provision a
    // stub, then require the editor page to render it (zero class chips,
    // label + role id visible).
    const prov = await request.post(`${apiBase}/api/raids/roles/provision`, {
      data: { paths: ['e2e-smoke-stub.mappings'] },
    })
    expect(prov.status()).toBe(200)

    try {
      await page.goto('/#/raids/editor', { waitUntil: 'domcontentloaded' })
      await expect(
        page.getByText('e2e-smoke-stub / mappings').first(),
      ).toBeVisible({ timeout: 15_000 })
    } finally {
      // The stub is unreferenced (no encounter was created) — deletable.
      await request.delete(
        `${apiBase}/api/raids/roles?role=e2e-smoke-stub&sub=mappings`,
      )
    }
  })
})
