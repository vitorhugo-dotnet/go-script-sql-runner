import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath } from 'node:url'

const editorWorkerPath = fileURLToPath(
  new URL('./editor/editor.worker.js', import.meta.resolve('monaco-editor')),
)

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: [
      {
        find: /^monaco-editor\/esm\/vs\/editor\/editor\.worker(?:\.js)?\?worker$/,
        replacement: `${editorWorkerPath}?worker`,
      },
    ],
  },
  worker: { format: 'es' },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
})
