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
import { expect, test } from '@playwright/test'

test.describe('app shell', () => {
  test('renders the shell and navigates to Raid Composition', async ({
    page,
  }) => {
    await page.goto('/#/', { waitUntil: 'domcontentloaded' })
    await expect(
      page.getByText('PQ Companion', { exact: true }).first(),
    ).toBeVisible()
    await page.getByRole('link', { name: 'Raid Composition' }).click()
    await expect(
      page.getByRole('heading', { name: 'Raid Composition Check' }),
    ).toBeVisible()
  })
})

test.describe('Raid Composition page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/#/raids', { waitUntil: 'domcontentloaded' })
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
})
