import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// Component tests run in jsdom. They mock the API module, so no server
// or network access is needed.
export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    // Process CSS so the Company page layout test can assert the raw
    // stylesheet (the shared 900px breakpoint) via `styles.css?raw`.
    css: true,
  },
})
