import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5174,
    proxy: {
      '/api/te': {
        target: process.env.VITE_DEV_API_TARGET ?? 'http://127.0.0.1:8081',
        changeOrigin: true,
      },
    },
  },
});
