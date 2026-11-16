import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// In development the Go API runs on 127.0.0.1:8080 and Vite proxies
// /api to it. In production the built dist/ directory is embedded in the
// Go binary (build with -tags ui).
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
