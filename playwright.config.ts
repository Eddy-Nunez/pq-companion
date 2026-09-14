// Playwright integration tests. Two projects:
//
//   api   — REST tests against a running Go backend. No browser needed; the
//           target is PQ_BASE_URL (default http://127.0.0.1:17654, the dev
//           handshake port). Start the backend first (win-smoke.ps1 does this
//           on Windows; any quarm.db-backed server works).
//
//   smoke — renderer smoke tests in plain Chromium. electron-vite has no
//           renderer-only mode (its dev command also launches Electron), so
//           the webServer below serves the frontend with vite.e2e.config.ts —
//           the same plugins/alias as the renderer section of
//           electron.vite.config.ts, on port 5174 to stay clear of a running
//           dev app on 5173. In a plain browser the app falls back to
//           http://127.0.0.1:17654 (frontend/src/services/backendUrl.ts), so
//           these tests also want the backend up.
//
// Run: npm run test:e2e        (smoke, starts its own vite)
//      npm run test:e2e:api    (api only, backend must be running)
import { defineConfig } from '@playwright/test'

const apiBase = process.env.PQ_BASE_URL ?? 'http://127.0.0.1:17654'

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  retries: 0,
  reporter: [['list']],
  use: {
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'api',
      testMatch: /api\/.*\.spec\.ts/,
      use: {
        baseURL: apiBase,
      },
    },
    {
      name: 'smoke',
      testMatch: /smoke\/.*\.spec\.ts/,
      use: {
        baseURL: 'http://localhost:5174',
      },
      webServer: process.env.PQ_NO_WEBSERVER
        ? undefined
        : {
            command: 'npx vite --config vite.e2e.config.ts --port 5174 --strictPort',
            url: 'http://localhost:5174',
            reuseExistingServer: true,
            timeout: 120_000,
          },
    },
  ],
})
