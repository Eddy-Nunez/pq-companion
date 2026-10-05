// Renderer smoke test for the Group Proposal page's shape integration
// (stage 4 of docs/raid-group-compositions-plan.md). Runs in plain Chromium
// against the vite-served frontend, backend on the dev port (17654).
//
// Writes a throwaway "e2e-split-shapes" encounter with two shapes via the
// API, then:
//   - shape toggles are HIDDEN in single-raid mode, visible in cohort mode
//   - checking a shape + generate renders "shape: <id>" badges on the
//     shape-formed group cards (replicate: one per raid)
//   - unchecked shapes generate a pure trinity proposal (no badges)
// The probe encounter is deleted at the end.
//
// Request-fixture calls MUST use absolute apiBase URLs: the smoke project's
// baseURL is the vite server (5174), which serves the frontend only — there
// is no /api proxy, so a relative request never reaches the backend (the
// page itself talks to 17654 directly via backendUrl.ts).
//
// The generate path uses the LIVE roster snapshot (the page never sends a
// manual roster), so this spec needs a backend that has seen a raid — the
// dev instance fed by zealsim. An empty roster still generates; the badge
// assertion then legitimately fails (a shape nobody can fill claims no
// group), which is the honest signal to re-feed the sim.
import { expect, test, type Page } from '@playwright/test'

const apiBase = process.env.PQ_BASE_URL ?? 'http://127.0.0.1:17654'
const ENC = 'e2e-split-shapes'

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
      shape_id: 'healstack',
      rows: [
        { role: 'healer', sub_role: 'ch_cleric', count: 2 },
        { role: 'tank', sub_role: 'defensive', count: 1 },
      ],
    },
    { shape_id: 'caststack', rows: [{ role: 'damage', count: 2 }] },
  ],
}

async function pickEncounter(page: Page): Promise<void> {
  const picker = page
    .locator('select')
    .filter({ has: page.locator('option', { hasText: 'E2E Split Shapes' }) })
    .first()
  await picker.selectOption({ value: ENC })
}

test.describe.serial('Group Proposal — group compositions', () => {
  let raidsWasEnabled = false

  test.beforeAll(async ({ request }) => {
    const cfg = await (await request.get(`${apiBase}/api/config`)).json()
    raidsWasEnabled = Boolean(cfg?.preferences?.raids_enabled)
    if (!raidsWasEnabled || !cfg.onboarding_completed) {
      cfg.preferences.raids_enabled = true
      cfg.onboarding_completed = true
      await request.put(`${apiBase}/api/config`, { data: cfg })
    }
    // Pre-clean leftovers from a failed earlier run (ignore errors).
    await request.delete(`${apiBase}/api/raids/encounters/${ENC}`)
    const res = await request.post(`${apiBase}/api/raids/encounters`, { data: ENC_BODY })
    expect(res.status()).toBe(201)
  })

  test.afterAll(async ({ request }) => {
    // Best-effort cleanup regardless of test outcomes.
    await request.delete(`${apiBase}/api/raids/encounters/${ENC}`)
    if (!raidsWasEnabled) {
      const cfg = await (await request.get(`${apiBase}/api/config`)).json()
      cfg.preferences.raids_enabled = false
      await request.put(`${apiBase}/api/config`, { data: cfg })
    }
  })

  test('toggles hidden in single-raid mode, visible in cohort mode', async ({ page }) => {
    await page.goto('/#/raids/split', { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('heading', { name: 'Group Composition Proposal' })).toBeVisible()
    await pickEncounter(page)
    await expect(page.getByText('group compositions')).toHaveCount(0)

    await page.getByLabel('split into').selectOption('2')
    await expect(page.getByText('group compositions')).toBeVisible()
    await expect(page.getByText('healstack', { exact: true })).toBeVisible()
    await expect(page.getByText('caststack', { exact: true })).toBeVisible()
    // Opt-in per generate: both start unchecked, placement defaults to replicate.
    await expect(page.getByLabel('placement')).toHaveValue('replicate')
    const heal = page.locator('label', { hasText: 'healstack' }).locator('input[type=checkbox]')
    await expect(heal).not.toBeChecked()
  })

  test('checking a shape renders shape-badged group cards in both raids', async ({ page }) => {
    await page.goto('/#/raids/split', { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('heading', { name: 'Group Composition Proposal' })).toBeVisible()
    await pickEncounter(page)
    await page.getByLabel('split into').selectOption('2')
    await page.locator('label', { hasText: 'healstack' }).locator('input[type=checkbox]').check()
    await page.getByRole('button', { name: 'Generate proposal' }).click()

    // Replicate: BOTH raids carry the shape group; each claims a template's
    // worth of seats (2 CH clerics + 1 defensive tank) with open seats left
    // open — the trinity pass never tops a shape group up.
    await expect(page.getByText('shape: healstack')).toHaveCount(2)
  })

  test('unchecked shapes generate a pure trinity proposal (no badges)', async ({ page }) => {
    await page.goto('/#/raids/split', { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('heading', { name: 'Group Composition Proposal' })).toBeVisible()
    await pickEncounter(page)
    await page.getByLabel('split into').selectOption('2')
    await page.getByRole('button', { name: 'Generate proposal' }).click()
    await expect(page.getByText(/Raid 1/).first()).toBeVisible()
    await expect(page.getByText(/^shape:/)).toHaveCount(0)
  })
})
