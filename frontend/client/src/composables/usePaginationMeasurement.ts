import { onScopeDispose, readonly, ref, watch, type DeepReadonly, type Ref } from 'vue';

type FrameHandle = number;

export interface PaginationFrameScheduler {
  cancelAnimationFrame(handle: FrameHandle): void;
  requestAnimationFrame(callback: FrameRequestCallback): FrameHandle;
}

export interface PaginationResizeObserver {
  disconnect(): void;
  observe(target: Element): void;
}

export interface PaginationMeasurementOptions {
  readonly fontReady?: Promise<unknown> | null;
  readonly frameScheduler?: PaginationFrameScheduler;
  readonly observerFactory?: (callback: ResizeObserverCallback) => PaginationResizeObserver | null;
}

export interface PaginationMeasurementController {
  readonly pages: readonly string[];
  dispose(): void;
  setContent(container: HTMLElement | null, text: string): void;
}

const defaultFrameScheduler: PaginationFrameScheduler = Object.freeze({
  cancelAnimationFrame: (handle: FrameHandle) => globalThis.cancelAnimationFrame(handle),
  requestAnimationFrame: (callback: FrameRequestCallback) => globalThis.requestAnimationFrame(callback),
});

function defaultObserverFactory(callback: ResizeObserverCallback): PaginationResizeObserver | null {
  return typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(callback);
}

function normalizedText(text: string): string {
  return text.replace(/\r\n?/gu, '\n');
}

function naturalPageBreak(text: string, start: number, fittedEnd: number): number {
  if (fittedEnd >= text.length) return text.length;
  const minimumBreak = start + Math.floor((fittedEnd - start) * 0.6);
  for (let index = fittedEnd; index > minimumBreak; index -= 1) {
    if (/\s/u.test(text[index - 1] ?? '')) return index;
  }
  return fittedEnd;
}

function replaceForMeasurement(container: HTMLElement, text: string): boolean {
  const original = [...container.childNodes];
  try {
    const fragment = document.createDocumentFragment();
    for (const line of text.split('\n')) {
      const row = document.createElement('div');
      row.textContent = line || '\u00a0';
      fragment.append(row);
    }
    container.replaceChildren(fragment);
    return container.scrollHeight <= container.clientHeight + 1 &&
      container.scrollWidth <= container.clientWidth + 1;
  } finally {
    container.replaceChildren(...original);
  }
}

export function paginateMeasuredText(container: HTMLElement, input: string): readonly string[] {
  const text = normalizedText(input);
  if (text === '') return Object.freeze(['']);
  if (container.clientHeight <= 0 || container.clientWidth <= 0) return Object.freeze([text]);
  const pages: string[] = [];
  let start = 0;
  while (start < text.length) {
    let low = start + 1;
    let high = text.length;
    let fittedEnd = start;
    while (low <= high) {
      const midpoint = Math.floor((low + high) / 2);
      if (replaceForMeasurement(container, text.slice(start, midpoint))) {
        fittedEnd = midpoint;
        low = midpoint + 1;
      } else {
        high = midpoint - 1;
      }
    }
    if (fittedEnd === start) fittedEnd = start + 1;
    const end = naturalPageBreak(text, start, fittedEnd);
    pages.push(text.slice(start, end));
    start = end;
  }
  return Object.freeze(pages);
}

export function createPaginationMeasurementController(
  options: PaginationMeasurementOptions = {},
  onPages?: (pages: readonly string[]) => void,
): PaginationMeasurementController {
  const frames = options.frameScheduler ?? defaultFrameScheduler;
  const observerFactory = options.observerFactory ?? defaultObserverFactory;
  const fontReady = options.fontReady === undefined
    ? (typeof document === 'undefined' ? null : document.fonts?.ready ?? null)
    : options.fontReady;
  let container: HTMLElement | null = null;
  let disposed = false;
  let frame: FrameHandle | null = null;
  let generation = 0;
  let observer: PaginationResizeObserver | null = null;
  let pages: readonly string[] = Object.freeze(['']);
  let text = '';

  const measure = (): void => {
    frame = null;
    if (disposed || container === null) return;
    pages = paginateMeasuredText(container, text);
    onPages?.(pages);
  };
  const schedule = (): void => {
    if (disposed || container === null) return;
    if (frame !== null) frames.cancelAnimationFrame(frame);
    frame = frames.requestAnimationFrame(measure);
  };
  const observe = (next: HTMLElement | null): void => {
    observer?.disconnect();
    observer = null;
    container = next;
    if (container !== null) {
      observer = observerFactory(schedule);
      observer?.observe(container);
    }
  };

  const fontGeneration = generation;
  void fontReady?.then(() => {
    if (!disposed && generation === fontGeneration) schedule();
  }).catch(() => undefined);

  return Object.freeze({
    get pages() { return pages; },
    dispose(): void {
      if (disposed) return;
      disposed = true;
      generation += 1;
      observer?.disconnect();
      observer = null;
      if (frame !== null) frames.cancelAnimationFrame(frame);
      frame = null;
      container = null;
    },
    setContent(nextContainer: HTMLElement | null, nextText: string): void {
      if (disposed) return;
      if (nextContainer !== container) observe(nextContainer);
      text = nextText;
      schedule();
    },
  });
}

export interface PaginationMeasurementComposable {
  readonly pages: DeepReadonly<Ref<readonly string[]>>;
  dispose(): void;
}

export function usePaginationMeasurement(
  container: Ref<HTMLElement | null>,
  text: Ref<string>,
  options: PaginationMeasurementOptions = {},
): PaginationMeasurementComposable {
  const pages = ref<readonly string[]>(Object.freeze(['']));
  const controller = createPaginationMeasurementController(options, next => { pages.value = next; });
  const stop = watch([container, text], ([nextContainer, nextText]) => {
    controller.setContent(nextContainer, nextText);
  }, { immediate: true });
  const dispose = (): void => {
    stop();
    controller.dispose();
  };
  onScopeDispose(dispose, true);
  return Object.freeze({ dispose, pages: readonly(pages) });
}
