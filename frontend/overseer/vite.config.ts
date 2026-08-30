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
  const candidate = mode === 'candidate';

  return {
    root: candidate ? 'test-fixtures' : 'src',
    base: './',
    resolve: {
      alias: {
        '#wails-service': fileURLToPath(new URL(
          './bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js',
          import.meta.url,
        )),
      },
    },
    plugins: [
      vue(),
      ...(candidate ? [] : [wails(fileURLToPath(new URL('./bindings', import.meta.url)))]),
      {
        name: 'preserve-legacy-entry-marker',
        transformIndexHtml: {
          order: 'pre',
          handler(html) {
            if (!html.includes('src="./overseer.js"')) {
              return html;
            }

            return html.replace('</head>', '  <!-- Vite source entry: overseer.js -->\n</head>');
          },
        },
      },
      preserveGoEmbedMarker(),
    ],
    build: {
      outDir: '../dist',
      emptyOutDir: true,
    },
  };
});
