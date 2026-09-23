import { writeFileSync } from 'node:fs'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'

// emptyOutDir deletes the tracked placeholder that lets Go embed dist before the UI is built.
const keepPlaceholder: Plugin = {
  name: 'keep-gitkeep',
  closeBundle() {
    writeFileSync(new URL('../internal/ui/dist/.gitkeep', import.meta.url), '')
  },
}

export default defineConfig({
  plugins: [react(), keepPlaceholder],
  build: {
    outDir: '../internal/ui/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
    },
  },
})
