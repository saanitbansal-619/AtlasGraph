import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// The dev server runs on :5173 — the origin the Go API pre-approves for CORS.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    host: true,
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    css: false,
  },
})
