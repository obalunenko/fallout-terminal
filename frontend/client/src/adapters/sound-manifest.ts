import { SoundCategory } from '../../gen/fallout/terminal/player/v1/sound_pb.js';
import type { PlayerRPCAdapter } from './player-rpc.js';

export const soundCategoryByFolder = Object.freeze({
  ambient: SoundCategory.AMBIENT,
  charscroll: SoundCategory.CHARSCROLL,
  enter: SoundCategory.ENTER,
  'hack-bad': SoundCategory.HACK_BAD,
  'hack-good': SoundCategory.HACK_GOOD,
  'menu-focus': SoundCategory.MENU_FOCUS,
  multiple: SoundCategory.MULTIPLE,
  single: SoundCategory.SINGLE,
} as const);

export type SoundFolder = keyof typeof soundCategoryByFolder;

export interface SoundManifestAdapter {
  load(folder: SoundFolder, signal?: AbortSignal): Promise<readonly string[]>;
}

const filenamePattern = /^[A-Za-z0-9_-]+\.(?:m4a|mp3|ogg|wav|webm)$/u;

export function safeSoundAssetURL(folder: SoundFolder, asset: unknown, origin = location.origin): string | null {
  if (typeof asset !== 'string') return null;
  const prefix = `sounds/${folder}/`;
  if (!asset.startsWith(prefix)) return null;
  const filename = asset.slice(prefix.length);
  if (!filenamePattern.test(filename) || filename.includes('/') || filename.includes('\\')) return null;
  let base: URL;
  try {
    base = new URL(origin);
  } catch {
    return null;
  }
  if (base.protocol !== 'http:' && base.protocol !== 'https:') return null;
  const url = new URL(`/${prefix}${encodeURIComponent(filename)}`, base);
  return url.origin === base.origin ? `${url.pathname}${url.search}${url.hash}` : null;
}

export function createSoundManifestAdapter(
  rpc: Pick<PlayerRPCAdapter, 'soundManifest'>,
  origin = location.origin,
): SoundManifestAdapter {
  const cache = new Map<SoundFolder, readonly string[]>();
  const pending = new Map<SoundFolder, Promise<readonly string[]>>();
  return Object.freeze({
    async load(folder: SoundFolder, signal?: AbortSignal): Promise<readonly string[]> {
      const cached = cache.get(folder);
      if (cached !== undefined) return cached;
      const active = pending.get(folder);
      if (active !== undefined) return active;
      const operation = (async (): Promise<readonly string[]> => {
        try {
          const input = { category: soundCategoryByFolder[folder] };
          const response = signal === undefined
            ? await rpc.soundManifest(input)
            : await rpc.soundManifest(input, { signal });
          const seen = new Set<string>();
          const assets: string[] = [];
          for (const candidate of response.assets) {
            const url = safeSoundAssetURL(folder, candidate, origin);
            if (url !== null && !seen.has(url)) {
              seen.add(url);
              assets.push(url);
            }
          }
          const result = Object.freeze(assets);
          cache.set(folder, result);
          return result;
        } catch {
          return Object.freeze([]);
        } finally {
          pending.delete(folder);
        }
      })();
      pending.set(folder, operation);
      return operation;
    },
  });
}
