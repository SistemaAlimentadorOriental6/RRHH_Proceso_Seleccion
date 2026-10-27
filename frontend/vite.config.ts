import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 3001,
    proxy: {
      '/api': {
        target: 'http://192.168.90.25:3002/api',
        changeOrigin: true
      }
    }
  }
});
