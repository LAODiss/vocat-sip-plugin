import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/plugin-assets/vocat-sipserver/',
  build: {
    outDir: '../assets',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        index: 'index.html',
        accounts: 'accounts.html',
        'call-log': 'call-log.html',
        settings: 'settings.html',
      },
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': 'http://localhost:8080',
    },
  },
})