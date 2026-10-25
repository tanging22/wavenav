import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// 构建产物直接进 internal/web/dist，由 go:embed 打进单二进制
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 800,
  },
  server: {
    port: 5179,
    proxy: {
      '/api': 'http://127.0.0.1:8019',
      '/uploads': 'http://127.0.0.1:8019',
    },
  },
})
