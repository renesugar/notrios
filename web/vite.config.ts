/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // The development address, matching config/config.example.yaml and
      // `make serve`. An installed Notrios keeps 8080; this proxy only ever
      // talks to the service running from this checkout.
      '/api': 'http://127.0.0.1:8099',
      '/healthz': 'http://127.0.0.1:8099'
    }
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false
  }
});
