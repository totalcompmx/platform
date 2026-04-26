import { resolve } from 'node:path';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  base: '/static/dist/',
  build: {
    emptyOutDir: true,
    manifest: true,
    outDir: '../assets/static/dist',
    rollupOptions: {
      input: {
        home: resolve(__dirname, 'src/entries/home.ts')
      }
    }
  },
  server: {
    cors: {
      origin: ['http://localhost:3080', 'http://127.0.0.1:3080']
    },
    fs: {
      allow: ['..']
    },
    host: '127.0.0.1',
    port: 5173,
    strictPort: true
  },
  test: {
    coverage: {
      exclude: ['src/entries/**', 'src/vite-env.d.ts'],
      include: ['src/home/**/*.ts'],
      provider: 'v8',
      reporter: ['text'],
      thresholds: {
        branches: 100,
        functions: 100,
        lines: 100,
        statements: 100
      }
    },
    environment: 'happy-dom',
    include: ['src/test/**/*.test.ts']
  }
});
