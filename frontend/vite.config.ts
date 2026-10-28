import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 3001,
    proxy: {
      '/api': {
        //target: 'https://procesos-disciplinarios.sao6.com.co/api',
        target: 'http://localhost:3002',
        changeOrigin: true
      }
    }
  }
});
