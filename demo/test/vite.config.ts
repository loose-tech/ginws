import { defineConfig } from 'vite'

export default defineConfig({
  server: {
    fs: {
      allow: ['..', '../../client'],
    },
    proxy: {
      '/ws': {
        target: 'ws://localhost:5000',
        ws: true,
      },
    },
  },
})
