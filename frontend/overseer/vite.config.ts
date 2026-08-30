import { writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import wails from '@wailsio/runtime/plugins/vite';
import vue from '@vitejs/plugin-vue';
import { defineConfig } from 'vite';

export default defineConfig({
  root: 'src',
  base: './',
  plugins: [
    vue(),
    wails(fileURLToPath(new URL('./bindings', import.meta.url))),
    {
      name: 'preserve-go-embed-marker',
      closeBundle() {
        writeFileSync(new URL('./dist/.keep', import.meta.url), '');
      },
    },
  ],
  build: {
    outDir: '../dist',
    emptyOutDir: true,
  },
});
