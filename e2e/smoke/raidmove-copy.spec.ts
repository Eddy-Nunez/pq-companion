// Renderer smoke test for the group card "copy #raidmove" button. Runs in
// plain Chromium against the vite-served frontend, backend on the dev port.
//
// Writes a throwaway encounter via the API, generates a proposal on the
// split page, clicks the card's #raidmove button, and asserts the clipboard
// holds one `#raidmove <member> <group>` line per seated member (in slot
// order). Clipboard access in a plain-Chromium context needs the
// clipboard-read permission granted per-context (use.options below).
//
// Uses the LIVE roster snapshot for the generate — same precondition as
// split-shapes.spec.ts (a zealsim-fed backend); an empty roster still
// generates but produces only trinity groups (still fine: every card
// copies its seated members).
import { expect, test, type Page } from '@playwright/test'

const apiBase = process.env.PQ_BASE_URL ?? 'http://127.0.0.1:17654'
const ENC = 'e2e-raidmove'

const ENC_BODY = {
  id: ENC,
  name: 'E2E Raidmove',
  zone: 'Kael Drakkel',
  zone_id: 113,
  status: 'active',
  comps: [
    { role: 'tank', sub_role: 'defensive', min: 1, rec: 1 },
    { role: 'healer', sub_role: 'ch_cleric', min: 2, rec: 2 },
    { role: 'damage', min: 3, rec: 3 },
  ],
}

async function pickEncounter(page: Page): Promise<void> {
  const picker = page
    .locator('select')
    .filter({ has: page.locator('option', { hasText: 'E2E Raidmove' }) })
    .first()
  await picker.selectOption({ value: ENC })
}

test.describe.serial('Group Proposal — #raidmove copy', () => {
  let raidsWasEnabled = false

  test.use({ contextOptions: { permissions: ['clipboard-read', 'clipboard-write'] } })

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

  test('copies one #raidmove line per member with the card group number', async ({ page }) => {
    await page.goto('/#/raids/split', { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('heading', { name: 'Group Composition Proposal' })).toBeVisible()
    await pickEncounter(page)
    await page.getByRole('button', { name: 'Generate proposal' }).click()
    await expect(page.getByText(/Group 1/).first()).toBeVisible()

    // First card's button: read its group number from the card title, click,
    // and check the clipboard.
    const card = page.locator('div.rounded-lg', { has: page.getByText('Group 1', { exact: true }) }).first()
    await card.getByRole('button', { name: '#raidmove' }).click()
    await expect(card.getByRole('button', { name: 'copied' })).toBeVisible()

    const text = await page.evaluate(() => navigator.clipboard.readText())
    // Windows clipboards normalize \n to \r\n — strip CRs and trailing blanks.
    const lines = text.replace(/\r/g, '').trim().split('\n').filter(Boolean)
    expect(lines.length).toBeGreaterThan(0)
    for (const line of lines) expect(line).toMatch(/^#raidmove \S+ 1$/)
    // Round-trip: the first copied line is the card's first seat row member.
    // (The name span also carries the Group Leader badge text when slot 1 is
    // the leader seat — scope to the span's direct text node content via the
    // clipboard line itself, and just assert the copied names appear in the
    // card's rows.)
    const cardText = await card.textContent()
    for (const line of lines) {
      const name = line.replace(/^#raidmove /, '').replace(/ 1$/, '')
      expect(cardText).toContain(name)
    }
  })

  test('copy-all button emits one line per seated member across every group', async ({ page }) => {
    await page.goto('/#/raids/split', { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('heading', { name: 'Group Composition Proposal' })).toBeVisible()
    await pickEncounter(page)
    await page.getByRole('button', { name: 'Generate proposal' }).click()
    await expect(page.getByRole('button', { name: 'copy all #raidmove' })).toBeVisible()
    await page.getByRole('button', { name: 'copy all #raidmove' }).click()
    await expect(page.getByRole('button', { name: 'copied' }).first()).toBeVisible()

    const text = await page.evaluate(() => navigator.clipboard.readText())
    const lines = text.replace(/\r/g, '').trim().split('\n').filter(Boolean)
    // Every line is a well-formed command; group numbers span 1..N.
    for (const line of lines) expect(line).toMatch(/^#raidmove \S+ \d+$/)
    // One line per seated member (names unique across the raid).
    const names = lines.map((l) => l.replace(/^#raidmove /, '').replace(/ \d+$/, ''))
    expect(new Set(names).size).toBe(lines.length)
    // Bench members (if any) ride at group 0 — the last lines.
    const benchLine = lines.find((l) => l.endsWith(' 0'))
    if (benchLine) expect(lines.indexOf(benchLine)).toBe(lines.length - 1)
  })

  test('copy reads CURRENT state: script follows a drag-and-drop edit', async ({ page }) => {
    // Regression guard for the stale-closure class: drag a member to
    // another group, then copy — the script must reflect the moved seat,
    // not the originally generated one.
    await page.goto('/#/raids/split', { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('heading', { name: 'Group Composition Proposal' })).toBeVisible()
    await pickEncounter(page)
    await page.getByRole('button', { name: 'Generate proposal' }).click()
    await expect(page.getByText(/Group 1/).first()).toBeVisible()
    expect(await page.getByRole('button', { name: 'copy all #raidmove' }).count()).toBe(1)

    // Pre-edit script for reference.
    await page.getByRole('button', { name: 'copy all #raidmove' }).click()
    await expect(page.getByRole('button', { name: 'copied' }).first()).toBeVisible()
    const before = (await page.evaluate(() => navigator.clipboard.readText())).replace(/\r/g, '')

    // Drag group 1's first member onto a member of group 2 — a SWAP, because
    // a full group (6/6) has no open-seat target to insert into. dnd-kit
    // PointerSensor needs a real gesture: press, move ≥6px to activate, then
    // travel to the target and release.
    const g1 = page.locator('div.rounded-lg', { has: page.getByText('Group 1', { exact: true }) }).first()
    const g2 = page.locator('div.rounded-lg', { has: page.getByText('Group 2', { exact: true }) }).first()
    // The copied script's first line is group 1's first seat — the member we
    // drag. Derive the bare name from the script: the row's name span also
    // carries the Group Leader badge text.
    const movedName = before.trim().split('\n')[0].replace(/^#raidmove /, '').replace(/ \d+$/, '')
    const source = g1.locator('span.truncate').filter({ hasText: movedName }).first()
    const target = g2.locator('span.truncate').first()
    const box = await source.boundingBox()
    await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2)
    await page.mouse.down()
    await page.mouse.move(box!.x + box!.width / 2 + 12, box!.y + box!.height / 2) // cross the 6px activation
    await target.hover({ steps: 8 })
    await page.mouse.up()
    await expect(page.getByText('manually adjusted')).toBeVisible({ timeout: 10_000 })

    // Copy again: the dragged member must now sit in group 2, and the script
    // must differ from the pre-edit one — the copy reads the proposal's
    // CURRENT state (post-drag), never the originally generated snapshot.
    await page.getByRole('button', { name: 'copy all #raidmove' }).click()
    await expect(page.getByRole('button', { name: 'copied' }).first()).toBeVisible()
    const after = (await page.evaluate(() => navigator.clipboard.readText())).replace(/\r/g, '')
    expect(after).not.toBe(before)
    const moved = after.trim().split('\n').find((l) => l.includes(`#raidmove ${movedName} `))
    expect(moved).toBeTruthy()
    expect(moved!.endsWith(' 2')).toBe(true)
  })
})
