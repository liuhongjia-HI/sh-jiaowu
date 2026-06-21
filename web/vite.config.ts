import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  base: '/',
  plugins: [react()],
  build: {
    chunkSizeWarningLimit: 700
  },
  server: {
    proxy: {
      '/api': `http://127.0.0.1:${process.env.HTTP_PORT ?? '8892'}`
    }
  }
});
