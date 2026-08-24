'use strict';

import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { PlayerService } from './gen/fallout/terminal/player/v1/player_pb.js';
import { SoundCategory } from './gen/fallout/terminal/player/v1/sound_pb.js';

// ════════════════════════════════════════════════════
// SOUND SYSTEM — Web Audio playback + folder-based random pick.
// Folders with a single expected file (ambient, hack-good, hack-bad,
// menu-focus) just play files[0]. Folders with many files (single,
// multiple, enter) pick a random one each time.
// ════════════════════════════════════════════════════

const folderFiles = {};   // folder -> [safe same-origin URLs]
const rawBufs     = new Map(); // url -> ArrayBuffer
const decodedBufs = new Map(); // url -> AudioBuffer
const folderLoads = new Map(); // folder -> Promise<[filenames]>
const rawLoads    = new Map(); // url -> Promise<void>
const oneShotFolders = ['single', 'multiple', 'enter', 'hack-good', 'hack-bad', 'menu-focus', 'charscroll'];
let audioCtx = null;
let webAudioEligible = false;
let webAudioReady = null;
const authoritativeCueKeys = new Set();
const soundRPC = createClient(PlayerService, createConnectTransport({
  baseUrl: window.location.origin,
  useBinaryFormat: true,
}));
const soundCategories = Object.freeze({
  ambient: SoundCategory.AMBIENT,
  'hack-good': SoundCategory.HACK_GOOD,
  'hack-bad': SoundCategory.HACK_BAD,
  'menu-focus': SoundCategory.MENU_FOCUS,
  single: SoundCategory.SINGLE,
  multiple: SoundCategory.MULTIPLE,
  enter: SoundCategory.ENTER,
  charscroll: SoundCategory.CHARSCROLL,
});
const supportedSoundExtension = /\.(?:mp3|wav|ogg|m4a|webm)$/i;
const safeSoundAssetURL = /^\/sounds\/[a-z-]+\/[^/]+$/;
const maxSoundDiagnosticEvents = 256;
let soundDiagnosticEvents = 0;

// shouldPlayAuthoritativeCue deduplicates one-shot effects by newly applied
// authoritative revision. Snapshots/reconnect baselines and rejected/replayed
// unary results never call this gate, so they cannot replay outcome audio.
function shouldPlayAuthoritativeCue(revision, cue) {
  const numericRevision = Number(revision);
  if (!Number.isFinite(numericRevision) || numericRevision <= 0 || !cue) return false;
  const key = `${numericRevision}:${cue}`;
  if (authoritativeCueKeys.has(key)) return false;
  authoritativeCueKeys.add(key);
  if (authoritativeCueKeys.size > 512) {
    const oldest = authoritativeCueKeys.values().next().value;
    authoritativeCueKeys.delete(oldest);
  }
  return true;
}

window.__falloutTerminalShouldPlayAuthoritativeCue = shouldPlayAuthoritativeCue;

function reportPlayback(url) {
  try {
    if (typeof window.__falloutTerminalSoundObserver === 'function') {
      window.__falloutTerminalSoundObserver(url);
    }
  } catch {
    // Test/diagnostic observation is optional and cannot affect playback.
  }
}

function reportSoundDiagnostic(stage, { folder = '', url = '' } = {}) {
  if (typeof window.__falloutTerminalSoundDiagnosticObserver !== 'function') return;
  if (soundDiagnosticEvents >= maxSoundDiagnosticEvents) return;
  soundDiagnosticEvents += 1;
  try {
    window.__falloutTerminalSoundDiagnosticObserver({
      stage,
      folder: Object.prototype.hasOwnProperty.call(soundCategories, folder) ? folder : '',
      url: safeSoundAssetURL.test(url) ? url : '',
    });
  } catch {
    // Test/diagnostic observation is optional and cannot affect playback.
  }
}

function getCtx() {
  const AudioContextClass = window.AudioContext || window.webkitAudioContext;
  if (!AudioContextClass) {
    reportSoundDiagnostic('context-unavailable');
    return null;
  }
  try {
    if (!audioCtx) audioCtx = new AudioContextClass();
  } catch {
    reportSoundDiagnostic('context-construction-failed');
    return null;
  }
  return audioCtx;
}

async function decodeRaw(context, url, raw) {
  reportSoundDiagnostic('decode-start', { url });
  try {
    const buffer = await context.decodeAudioData(raw.slice(0));
    reportSoundDiagnostic('decode-ready', { url });
    return buffer;
  } catch (error) {
    reportSoundDiagnostic('decode-failed', { url });
    throw error;
  }
}

function enableWebAudio() {
  if (webAudioEligible) return Promise.resolve(true);
  if (webAudioReady) return webAudioReady;
  const context = getCtx();
  if (!context) return null;

  const attempt = (async () => {
    try {
      if (context.state === 'suspended') {
        reportSoundDiagnostic('resume-start');
        await context.resume();
        reportSoundDiagnostic('resume-ready');
      }
      if (context.state !== 'running') {
        reportSoundDiagnostic('context-not-running');
        return false;
      }

      await Promise.all(oneShotFolders.map(loadFolder));
      await Promise.all(Array.from(rawBufs.entries()).map(async ([url, raw]) => {
        if (decodedBufs.has(url)) return;
        const buffer = await decodeRaw(context, url, raw);
        decodedBufs.set(url, buffer);
      }));
      if (context.state === 'running') {
        webAudioEligible = true;
        return true;
      }
    } catch {
      reportSoundDiagnostic('audio-enable-failed');
      // A document without an eligible Web Audio context remains silent.
    }
    return false;
  })();
  webAudioReady = attempt;
  attempt.then(ready => {
    if (!ready && webAudioReady === attempt) webAudioReady = null;
  });
  return attempt;
}

async function prefetch(url) {
  if (rawBufs.has(url) || decodedBufs.has(url)) return;
  if (rawLoads.has(url)) return rawLoads.get(url);
  const loading = (async () => {
    reportSoundDiagnostic('fetch-start', { url });
    try {
      const res = await fetch(url);
      if (!res.ok) {
        reportSoundDiagnostic('fetch-failed', { url });
        return;
      }
      rawBufs.set(url, await res.arrayBuffer());
      reportSoundDiagnostic('fetch-ready', { url });
    } catch {
      reportSoundDiagnostic('fetch-failed', { url });
    }
  })();
  rawLoads.set(url, loading);
  try {
    await loading;
  } finally {
    rawLoads.delete(url);
  }
}

async function playBuf(url, volume) {
  try {
    if (!webAudioReady || !await webAudioReady || !webAudioEligible) {
      reportSoundDiagnostic('playback-ineligible', { url });
      return;
    }
    let buffer = decodedBufs.get(url);
    if (!buffer) {
      if (!rawBufs.has(url)) await prefetch(url);
      const raw = rawBufs.get(url);
      if (!raw) return;
      const c = getCtx();
      if (!c) return;
      buffer = await decodeRaw(c, url, raw);
      decodedBufs.set(url, buffer);
    }
    const c = getCtx();
    if (!c) return;
    const src = c.createBufferSource();
    const gain = c.createGain();
    src.buffer = buffer;
    gain.gain.value = volume;
    src.connect(gain);
    gain.connect(c.destination);
    try {
      src.start();
    } catch {
      reportSoundDiagnostic('source-start-failed', { url });
      return;
    }
    reportSoundDiagnostic('source-started', { url });
    reportPlayback(url);
  } catch {
    reportSoundDiagnostic('playback-failed', { url });
    // Audio is optional: decode, autoplay, and device failures are non-fatal.
  }
}

async function loadFolder(name) {
  if (Object.prototype.hasOwnProperty.call(folderFiles, name)) return folderFiles[name];
  if (folderLoads.has(name)) return folderLoads.get(name);
  const loading = (async () => {
    reportSoundDiagnostic('manifest-start', { folder: name });
    try {
      const category = soundCategories[name];
      if (category === undefined) return [];
      const manifest = await soundRPC.soundManifest({ category });
      const prefix = `sounds/${name}/`;
      const supported = (manifest.assets || []).filter(asset => {
        if (typeof asset !== 'string' || !asset.startsWith(prefix) || !supportedSoundExtension.test(asset)) return false;
        const filename = asset.slice(prefix.length);
        return filename.length > 0 && !filename.includes('/') && !filename.includes('\\') && filename !== '.' && filename !== '..';
      }).map(asset => `/${prefix}${encodeURIComponent(asset.slice(prefix.length))}`);
      folderFiles[name] = supported;
      await Promise.all(supported.map(prefetch));
      reportSoundDiagnostic('manifest-ready', { folder: name });
      return supported;
    } catch {
      reportSoundDiagnostic('manifest-failed', { folder: name });
      return [];
    }
  })();
  folderLoads.set(name, loading);
  try {
    return await loading;
  } finally {
    folderLoads.delete(name);
  }
}

async function playFromFolder(name, volume) {
  reportSoundDiagnostic('dispatch', { folder: name });
  const files = folderFiles[name] || await loadFolder(name);
  if (!files || !files.length) {
    reportSoundDiagnostic('asset-unavailable', { folder: name });
    return;
  }
  const url = files[Math.floor(Math.random() * files.length)];
  await playBuf(url, volume);
}

async function playFirst(name, volume) {
  reportSoundDiagnostic('dispatch', { folder: name });
  const files = folderFiles[name] || await loadFolder(name);
  if (!files || !files.length) {
    reportSoundDiagnostic('asset-unavailable', { folder: name });
    return;
  }
  await playBuf(files[0], volume);
}

// ── Public one-shot sounds ─────────────────────────────
export function playSingle()    { playFromFolder('single', 0.55); }
export function playMultiple()  { playFromFolder('multiple', 0.55); }
export function playEnter()     { playFromFolder('enter', 0.65); }
export function playMenuFocus() { playFirst('menu-focus', 0.5); }
export function playHackGood()  { playFirst('hack-good', 0.8); }
export function playHackBad()   { playFirst('hack-bad', 0.7); }
export function playCharScroll() { playFromFolder('charscroll', 0.4); }

// ── Ambient loop (requested by accepted terminal lifecycle state) ───
let ambientAudio = null;
let ambientReady = false;
let ambientRequested = false;
let ambientRevision = 0;
let ambientPlayAttempt = null;

async function setupAmbient() {
  try {
    await loadFolder('ambient');
    const files = folderFiles.ambient;
    if (!files || !files.length || typeof window.Audio !== 'function') return;
    ambientAudio = new window.Audio(files[0]);
    ambientAudio.loop = true;
    ambientAudio.volume = 0.25;
    ambientReady = true;
    reconcileAmbient();
  } catch {
    ambientAudio = null;
    ambientReady = false;
  }
}

function reconcileAmbient() {
  if (!ambientRequested || !ambientReady || !ambientAudio ||
      !ambientAudio.paused || ambientPlayAttempt) return;

  const revision = ambientRevision;
  try {
    const playing = ambientAudio.play();
    if (!playing || typeof playing.then !== 'function') return;

    const attempt = Promise.resolve(playing)
      .then(() => {
        if (!ambientRequested || revision !== ambientRevision) stopAmbient();
      })
      .catch(() => {
        // Keep the request active so a later qualifying gesture can retry it.
      })
      .finally(() => {
        if (ambientPlayAttempt === attempt) ambientPlayAttempt = null;
      });
    ambientPlayAttempt = attempt;
  } catch {
    // Autoplay and device failures are optional and retryable.
  }
}

export function setAmbientActive(active) {
  const requested = Boolean(active);
  if (requested !== ambientRequested) {
    ambientRequested = requested;
    ambientRevision += 1;
  }
  if (ambientRequested) {
    reconcileAmbient();
  } else {
    stopAmbient();
  }
}

function stopAmbient() {
  try {
    if (ambientAudio) ambientAudio.pause();
  } catch { /* optional audio must not interrupt terminal state */ }
}

function handleAudioGesture() {
  enableWebAudio();
  reconcileAmbient();
}

document.addEventListener('pointerdown', handleAudioGesture);
document.addEventListener('keydown', handleAudioGesture);

// ── Boot: prefetch everything up front ─────────────────
oneShotFolders.forEach(loadFolder);
setupAmbient().catch(() => {});
