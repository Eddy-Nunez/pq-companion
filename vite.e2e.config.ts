// Renderer-only vite config for the Playwright smoke tests. Mirrors the
// renderer section of electron.vite.config.ts (root, plugins, alias,
// optimizeDeps) — kept in sync by hand, because electron-vite's dev command
// always launches Electron and cannot serve just the renderer for tests.
// The port is pinned to 5174 so this never collides with a running dev app.
import { defineConfig } from 'vite'
import { resolve } from 'path'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  root: resolve(__dirname, 'frontend'),
  plugins: [tailwindcss(), react()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'frontend/src'),
    },
  },
  optimizeDeps: {
    include: ['@xyflow/react', 'dagre', 'lucide-react', 'react-router-dom'],
  },
  server: {
    port: 5174,
    strictPort: true,
  },
})
