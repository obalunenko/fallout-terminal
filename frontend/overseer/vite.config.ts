import { writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import wails from '@wailsio/runtime/plugins/vite';
import vue from '@vitejs/plugin-vue';
import { defineConfig, type Plugin } from 'vite';

function preserveGoEmbedMarker(): Plugin {
  let outputDirectory: string;

  return {
    name: 'preserve-go-embed-marker',
    configResolved(config) {
      outputDirectory = resolve(config.root, config.build.outDir);
    },
    closeBundle() {
      writeFileSync(resolve(outputDirectory, '.keep'), '');
    },
  };
}

export default defineConfig(({ mode }) => {
  const browserTest = mode === 'browser-test';
  const productionRoot = fileURLToPath(new URL('./src', import.meta.url)); // root: frontend/overseer/src
  const browserBindings = fileURLToPath(new URL('../../tests/browser/fixtures/desktop-bindings.js', import.meta.url));

  return {
    root: productionRoot,
    base: './',
    resolve: {
      alias: {
        '#wails-service': browserTest
          ? browserBindings
          : fileURLToPath(new URL(
            './bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js',
            import.meta.url,
          )),
        ...(browserTest ? { '@wailsio/runtime': browserBindings } : {}),
      },
    },
    plugins: [
      vue(),
      ...(browserTest ? [] : [wails(fileURLToPath(new URL('./bindings', import.meta.url)))]),
      preserveGoEmbedMarker(),
    ],
    server: {
      fs: {
        allow: [fileURLToPath(new URL('../..', import.meta.url))],
      },
    },
    build: {
      outDir: '../dist',
      emptyOutDir: true,
    },
  };
});
