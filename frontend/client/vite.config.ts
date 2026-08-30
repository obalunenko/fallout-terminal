import { cpSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import vue from '@vitejs/plugin-vue';
import { defineConfig, type Plugin } from 'vite';

function copyEmbeddedSoundAssets(): Plugin {
  let outputDirectory: string;

  return {
    name: 'copy-embedded-sound-assets',
    configResolved(config) {
      outputDirectory = resolve(config.root, config.build.outDir);
    },
    closeBundle() {
      cpSync(fileURLToPath(new URL('./sounds', import.meta.url)), resolve(outputDirectory, 'sounds'), {
        recursive: true,
      });
      writeFileSync(resolve(outputDirectory, '.keep'), '');
    },
  };
}

export default defineConfig({
  plugins: [vue(), copyEmbeddedSoundAssets()],
});
