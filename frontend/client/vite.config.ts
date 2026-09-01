import { cpSync, mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import vue from '@vitejs/plugin-vue';
import { defineConfig, type Plugin } from 'vite';

const embeddedSoundAssets = [
  'ambient/obj_computerzax_hum_lp.wav',
  'charscroll/ui_hacking_charscroll.wav',
  'enter/ui_hacking_charenter_01.wav',
  'enter/ui_hacking_charenter_02.wav',
  'enter/ui_hacking_charenter_03.wav',
  'hack-bad/ui_hacking_passbad.wav',
  'hack-good/ui_hacking_passgood.wav',
  'menu-focus/ui_menu_focus.wav',
  'multiple/ui_hacking_charmultiple_01.wav',
  'multiple/ui_hacking_charmultiple_02.wav',
  'multiple/ui_hacking_charmultiple_03.wav',
  'multiple/ui_hacking_charmultiple_04.wav',
  'single/ui_hacking_charsingle_01.wav',
  'single/ui_hacking_charsingle_02.wav',
  'single/ui_hacking_charsingle_03.wav',
  'single/ui_hacking_charsingle_04.wav',
  'single/ui_hacking_charsingle_05.wav',
  'single/ui_hacking_charsingle_06.wav',
  'single/ui_hacking_charsingle_07.wav',
  'single/ui_hacking_charsingle_08.wav',
] as const;

function copyEmbeddedSoundAssets(): Plugin {
  let outputDirectory: string;

  return {
    name: 'copy-embedded-sound-assets',
    configResolved(config) {
      outputDirectory = resolve(config.root, config.build.outDir);
    },
    closeBundle() {
      for (const asset of embeddedSoundAssets) {
        const destination = resolve(outputDirectory, 'sounds', asset);
        mkdirSync(resolve(destination, '..'), { recursive: true });
        cpSync(fileURLToPath(new URL(`./sounds/${asset}`, import.meta.url)), destination);
      }
      writeFileSync(resolve(outputDirectory, '.keep'), '');
    },
  };
}

export default defineConfig(() => {
  // Production input: frontend/client/index.html; root: frontend/client.
  const clientRoot = fileURLToPath(new URL('.', import.meta.url));

  return {
    root: clientRoot,
    base: './',
    plugins: [vue(), copyEmbeddedSoundAssets()],
    server: {
      fs: {
        allow: [
          fileURLToPath(new URL('..', import.meta.url)),
          fileURLToPath(new URL('../../tests/browser/fixtures', import.meta.url)),
        ],
      },
    },
  };
});
