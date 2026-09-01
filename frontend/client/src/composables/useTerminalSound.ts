import { onScopeDispose } from 'vue';

import type { SoundFolder, SoundManifestAdapter } from '../adapters/sound-manifest.js';

export type TerminalSoundCue = 'charscroll' | 'enter' | 'hack-bad' | 'hack-good' | 'menu-focus' | 'multiple' | 'single';

interface AudioNodeLike {
  connect(destination: unknown): unknown;
  disconnect(): void;
}

interface BufferSourceLike extends AudioNodeLike {
  buffer: unknown;
  onended: ((event: Event) => void) | null;
  start(): void;
  stop(): void;
}

interface GainNodeLike extends AudioNodeLike {
  readonly gain: { value: number };
}

export interface TerminalAudioContext {
  readonly destination: unknown;
  readonly state: string;
  close(): Promise<unknown>;
  createBufferSource(): BufferSourceLike;
  createGain(): GainNodeLike;
  decodeAudioData(data: ArrayBuffer): Promise<unknown>;
  resume(): Promise<unknown>;
}

export interface AmbientAudio {
  loop: boolean;
  volume: number;
  pause(): void;
  play(): Promise<unknown> | void;
}

export interface TerminalSoundOptions {
  readonly ambientFactory?: (url: string) => AmbientAudio;
  readonly audioContextFactory?: () => TerminalAudioContext | null;
  readonly fetcher?: (url: string, init: { readonly signal: AbortSignal }) => Promise<{
    readonly ok: boolean;
    arrayBuffer(): Promise<ArrayBuffer>;
  }>;
  readonly gestureTarget?: EventTarget;
  readonly manifest: SoundManifestAdapter;
  readonly onPlayback?: (cue: TerminalSoundCue, url: string, volume: number) => void;
  readonly random?: () => number;
}

export interface TerminalSoundController {
  dispose(): Promise<void>;
  play(cue: TerminalSoundCue, revision?: number): Promise<boolean>;
  setAmbientActive(active: boolean): Promise<boolean>;
  unlock(): Promise<boolean>;
}

const cueConfiguration: Readonly<Record<TerminalSoundCue, Readonly<{
  folder: SoundFolder;
  random: boolean;
  volume: number;
}>>> = Object.freeze({
  charscroll: Object.freeze({ folder: 'charscroll', random: true, volume: 0.4 }),
  enter: Object.freeze({ folder: 'enter', random: true, volume: 0.65 }),
  'hack-bad': Object.freeze({ folder: 'hack-bad', random: false, volume: 0.7 }),
  'hack-good': Object.freeze({ folder: 'hack-good', random: false, volume: 0.8 }),
  'menu-focus': Object.freeze({ folder: 'menu-focus', random: false, volume: 0.5 }),
  multiple: Object.freeze({ folder: 'multiple', random: true, volume: 0.55 }),
  single: Object.freeze({ folder: 'single', random: true, volume: 0.55 }),
});

interface SoundDiagnosticGlobal {
  __falloutTerminalSoundDiagnosticObserver?: (event: Readonly<{
    folder: SoundFolder | '';
    stage: string;
    url: string;
  }>) => void;
  __falloutTerminalSoundObserver?: (url: string) => void;
}

function reportSoundDiagnostic(stage: string, folder: SoundFolder | '' = '', url = ''): void {
  try {
    (globalThis as typeof globalThis & SoundDiagnosticGlobal).__falloutTerminalSoundDiagnosticObserver?.({
      folder,
      stage,
      url,
    });
  } catch { /* optional diagnostics cannot affect playback */ }
}

function reportPlayback(url: string): void {
  try {
    (globalThis as typeof globalThis & SoundDiagnosticGlobal).__falloutTerminalSoundObserver?.(url);
  } catch { /* optional diagnostics cannot affect playback */ }
}

function defaultContextFactory(): TerminalAudioContext | null {
  const Context = globalThis.AudioContext;
  return typeof Context === 'function' ? new Context() : null;
}

function defaultAmbientFactory(url: string): AmbientAudio {
  return new Audio(url);
}

export function createTerminalSoundController(options: TerminalSoundOptions): TerminalSoundController {
  const ambientFactory = options.ambientFactory ?? defaultAmbientFactory;
  const contextFactory = options.audioContextFactory ?? defaultContextFactory;
  const fetcher = options.fetcher ?? ((url, init) => fetch(url, init));
  const gestureTarget = options.gestureTarget ?? document;
  const random = options.random ?? Math.random;
  const abortControllers = new Set<AbortController>();
  const activeNodes = new Map<BufferSourceLike, GainNodeLike>();
  const cueKeys = new Set<string>();
  const operations = new Set<Promise<unknown>>();
  let ambient: AmbientAudio | null = null;
  let ambientGeneration = 0;
  let ambientRequested = false;
  let context: TerminalAudioContext | null = null;
  let disposed = false;
  let unlockPromise: Promise<boolean> | null = null;

  const track = <T>(operation: Promise<T>): Promise<T> => {
    operations.add(operation);
    void operation.finally(() => operations.delete(operation));
    return operation;
  };
  const unlock = async (): Promise<boolean> => {
    if (disposed) return false;
    if (context?.state === 'running') return true;
    if (unlockPromise !== null) return unlockPromise;
    const operation = (async (): Promise<boolean> => {
      try {
        context ??= contextFactory();
        if (context === null) return false;
        if (context.state === 'suspended') await context.resume();
        return !disposed && context.state === 'running';
      } catch {
        return false;
      } finally {
        unlockPromise = null;
      }
    })();
    unlockPromise = operation;
    return track(operation);
  };
  const gesture = (): void => { void unlock(); };
  gestureTarget.addEventListener('pointerdown', gesture);
  gestureTarget.addEventListener('keydown', gesture);

  const play = (cue: TerminalSoundCue, revision?: number): Promise<boolean> => {
    if (disposed) return Promise.resolve(false);
    if (revision !== undefined) {
      if (!Number.isSafeInteger(revision) || revision <= 0) return Promise.resolve(false);
      const key = `${revision}:${cue}`;
      if (cueKeys.has(key)) return Promise.resolve(false);
      cueKeys.add(key);
      if (cueKeys.size > 512) cueKeys.delete(cueKeys.values().next().value ?? '');
    }
    reportSoundDiagnostic('dispatch', cueConfiguration[cue].folder);
    const operation = (async (): Promise<boolean> => {
      if (!await unlock() || disposed || context === null) return false;
      const config = cueConfiguration[cue];
      const controller = new AbortController();
      abortControllers.add(controller);
      try {
        const urls = await options.manifest.load(config.folder, controller.signal);
        if (disposed || controller.signal.aborted || urls.length === 0) return false;
        const index = config.random ? Math.min(urls.length - 1, Math.floor(random() * urls.length)) : 0;
        const url = urls[index];
        if (url === undefined) return false;
        const response = await fetcher(url, { signal: controller.signal });
        if (!response.ok || disposed || controller.signal.aborted) return false;
        const buffer = await context.decodeAudioData(await response.arrayBuffer());
        if (disposed || controller.signal.aborted) return false;
        const source = context.createBufferSource();
        const gain = context.createGain();
        source.buffer = buffer;
        gain.gain.value = config.volume;
        source.connect(gain);
        gain.connect(context.destination);
        activeNodes.set(source, gain);
        source.onended = () => {
          activeNodes.delete(source);
          source.disconnect();
          gain.disconnect();
        };
        source.start();
        reportSoundDiagnostic('source-started', config.folder, url);
        reportPlayback(url);
        options.onPlayback?.(cue, url, config.volume);
        return true;
      } catch {
        return false;
      } finally {
        abortControllers.delete(controller);
      }
    })();
    return track(operation);
  };

  return Object.freeze({
    async dispose(): Promise<void> {
      if (disposed) return;
      disposed = true;
      gestureTarget.removeEventListener('pointerdown', gesture);
      gestureTarget.removeEventListener('keydown', gesture);
      ambientGeneration += 1;
      ambientRequested = false;
      for (const controller of abortControllers) controller.abort();
      abortControllers.clear();
      try { ambient?.pause(); } catch { /* optional audio teardown cannot escape */ }
      ambient = null;
      for (const [source, gain] of activeNodes) {
        try { source.stop(); } catch { /* already stopped */ }
        try { source.disconnect(); } catch { /* optional node teardown */ }
        try { gain.disconnect(); } catch { /* optional node teardown */ }
      }
      activeNodes.clear();
      await Promise.allSettled([...operations]);
      const closing = context;
      context = null;
      if (closing !== null) {
        try { await closing.close(); } catch { /* optional context teardown */ }
      }
    },
    play,
    async setAmbientActive(active: boolean): Promise<boolean> {
      if (disposed) return false;
      ambientRequested = active;
      const generation = ++ambientGeneration;
      if (!active) {
        try { ambient?.pause(); } catch { /* optional audio */ }
        return true;
      }
      try {
        const urls = await options.manifest.load('ambient');
        if (disposed || !ambientRequested || generation !== ambientGeneration || urls[0] === undefined) return false;
        ambient ??= ambientFactory(urls[0]);
        ambient.loop = true;
        ambient.volume = 0.25;
        await ambient.play();
        if (disposed || !ambientRequested || generation !== ambientGeneration) {
          ambient.pause();
          return false;
        }
        return true;
      } catch {
        return false;
      }
    },
    unlock,
  });
}

export function useTerminalSound(options: TerminalSoundOptions): TerminalSoundController {
  const controller = createTerminalSoundController(options);
  onScopeDispose(() => { void controller.dispose(); }, true);
  return controller;
}
