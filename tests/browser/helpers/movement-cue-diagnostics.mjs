export async function installMovementCueDiagnostics(target) {
  await target.addInitScript(() => {
    window.__movementCueURLs = [];
    window.__movementCueStages = [];
    window.__presentationStages = [];
    window.__falloutTerminalSoundObserver = url => window.__movementCueURLs.push(String(url));
    window.__falloutTerminalSoundDiagnosticObserver = event => window.__movementCueStages.push({ ...event });
    window.__falloutTerminalPresentationObserver = event => window.__presentationStages.push({ ...event });
    class ObservableAudioContext {
      state = 'running';
      destination = {};
      async resume() { this.state = 'running'; }
      async decodeAudioData() { return {}; }
      createBufferSource() { return { buffer: null, connect() {}, start() {} }; }
      createGain() { return { gain: { value: 0 }, connect() {} }; }
    }
    Object.defineProperty(window, 'AudioContext', { configurable: true, value: ObservableAudioContext });
    Object.defineProperty(window, 'webkitAudioContext', { configurable: true, value: ObservableAudioContext });
    HTMLMediaElement.prototype.play = () => Promise.resolve();
  });
}

export async function resetMovementCueDiagnostics(page) {
  await page.evaluate(() => {
    window.__movementCueURLs = [];
    window.__movementCueStages = [];
    window.__presentationStages = [];
  });
}

export async function movementCueDiagnostics(page) {
  return page.evaluate(() => ({
    urls: [...window.__movementCueURLs],
    stages: [...window.__movementCueStages],
    presentationStages: [...window.__presentationStages],
  }));
}
