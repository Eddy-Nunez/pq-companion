// Renderer smoke test for the Raid Editor "Group compositions" section
// (stage 3 of docs/raid-group-compositions-plan.md). Runs in plain Chromium
// against the vite-served frontend, backend on the dev port (17654).
//
// Writes a throwaway "stage3-probe" encounter into the real user.db and
// DELETES it at the end — the serial flow makes cleanup deterministic (the
// delete tests run last; a mid-run failure can leave the probe encounter
// behind, same trade-off the app-smoke spec accepts).
//
// Note: the shape CRUD needs the live taxonomy's leaves for its role
// selects; the taxonomy is provisioned on any store (SeedRoles default), so
// this works on pristine and real dbs alike.
import { expect, test } from '@playwright/test'

const apiBase = process.env.PQ_BASE_URL ?? 'http://127.0.0.1:17654'
const ENC_NAME = 'Stage3 Probe'

// One encounter row in the editor list (rounded-lg px-3 py-2 is the row's
// own class combo — 'div' alone matches every ancestor of the name span).
const listRow = (page: import('@playwright/test').Page) =>
  page.locator('div.rounded-lg.px-3.py-2', { hasText: ENC_NAME }).first()

test.describe.serial('Raid Editor — group compositions', () => {
  let raidsWasEnabled = false

  test.beforeAll(async ({ request }) => {
    const cfg = await (await request.get(`${apiBase}/api/config`)).json()
    raidsWasEnabled = Boolean(cfg?.preferences?.raids_enabled)
    if (!raidsWasEnabled || !cfg.onboarding_completed) {
      cfg.preferences.raids_enabled = true
      cfg.onboarding_completed = true
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

  test('creates an encounter with a group composition and persists it', async ({ page }) => {
    await page.goto('/#/raids/editor', { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('heading', { name: 'Raid Composition Editor' })).toBeVisible()

    await page.getByRole('button', { name: 'New Encounter' }).click()
    await expect(page.getByRole('heading', { name: 'New Raid Encounter' })).toBeVisible()

    await page.getByPlaceholder('Avatar of War').fill(ENC_NAME)

    // Zone type-ahead: type, then pick the catalog match.
    await page.getByPlaceholder('type a zone name…').fill('Kael')
    await page.getByRole('button', { name: /Kael Drakkel/ }).first().click()

    // Add one shape: healstack = 2 CH clerics + 1 defensive tank.
    await page.getByRole('button', { name: 'Add group composition' }).click()
    await page.getByLabel('shape identifier').fill('healstack')
    const rows = page.getByLabel('shape row role')
    await rows.first().selectOption({ value: 'healer.ch_cleric' })
    await page.getByLabel('shape row count').first().fill('2')
    await page.getByRole('button', { name: 'add row' }).click()
    await rows.nth(1).selectOption({ value: 'tank.defensive' })
    await page.getByLabel('shape row count').nth(1).fill('1')

    await page.getByRole('button', { name: 'Save Encounter' }).click()
    // Back to the list with the shapes chip visible.
    await expect(page.getByRole('heading', { name: 'Raid Composition Editor' })).toBeVisible()
    const row = listRow(page)
    await expect(row.getByText('1 shape')).toBeVisible()

    // Duplicate-id client guard: reopen the form and try a second shape
    // with the same id.
    await row.getByRole('button', { name: 'Edit' }).click()
    await expect(page.getByLabel('shape identifier')).toHaveValue('healstack')
    await expect(page.getByLabel('shape row role').first()).toHaveValue('healer.ch_cleric')
    await expect(page.getByLabel('shape row count').first()).toHaveValue('2')
    await expect(page.getByLabel('shape row role').nth(1)).toHaveValue('tank.defensive')
    await page.getByRole('button', { name: 'Add group composition' }).click()
    await page.getByLabel('shape identifier').nth(1).fill('healstack')
    await page.getByRole('button', { name: 'Save Encounter' }).click()
    await expect(page.getByText('Duplicate group composition id "healstack"')).toBeVisible()
  })

  test('edits the shape (count change) and re-persists it', async ({ page }) => {
    await page.goto('/#/raids/editor', { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('heading', { name: 'Raid Composition Editor' })).toBeVisible()
    const row = listRow(page)
    await row.getByRole('button', { name: 'Edit' }).click()
    await page.getByLabel('shape row count').first().fill('3')
    await page.getByRole('button', { name: 'Save Encounter' }).click()
    await expect(page.getByRole('heading', { name: 'Raid Composition Editor' })).toBeVisible()

    // Round-trip through the API — the storage layer is the source of truth.
    const enc = await (await fetch(`${apiBase}/api/raids/encounters/stage3-probe`)).json()
    expect(enc.shapes).toHaveLength(1)
    expect(enc.shapes[0].shape_id).toBe('healstack')
    expect(enc.shapes[0].rows).toEqual([
      { role: 'healer', sub_role: 'ch_cleric', count: 3 },
      { role: 'tank', sub_role: 'defensive', count: 1 },
    ])
  })

  test('removes the shape and deletes the probe encounter (cleanup)', async ({ page }) => {
    await page.goto('/#/raids/editor', { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('heading', { name: 'Raid Composition Editor' })).toBeVisible()
    const row = listRow(page)

    // Remove the shape in the form, save, verify the chip is gone.
    await row.getByRole('button', { name: 'Edit' }).click()
    await page.getByRole('button', { name: 'Remove this group composition' }).click()
    await page.getByRole('button', { name: 'Save Encounter' }).click()
    await expect(page.getByRole('heading', { name: 'Raid Composition Editor' })).toBeVisible()
    await expect(row.getByText('1 shape')).toHaveCount(0)

    // Delete the probe encounter so the real db is left as we found it.
    await row.getByRole('button', { name: 'Delete' }).click()
    await row.getByRole('button', { name: 'Confirm' }).click()
    await expect(row).toHaveCount(0)
    const res = await fetch(`${apiBase}/api/raids/encounters/stage3-probe`)
    expect(res.status).toBe(404)
  })
})
