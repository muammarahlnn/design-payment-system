import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    // `npm run dev` outside Docker still reaches the running stack.
    proxy: {
      '/api/payments': { target: 'http://localhost:8080', rewrite: (p) => p.replace(/^\/api\/payments/, '') },
      '/api/gateway':  { target: 'http://localhost:8082', rewrite: (p) => p.replace(/^\/api\/gateway/, '') },
    },
  },
});
