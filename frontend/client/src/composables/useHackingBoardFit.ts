import { onScopeDispose, watch, type Ref } from 'vue';

type FrameHandle = number;

export interface HackingBoardFit {
  readonly compact: boolean;
  readonly fontSize: number | null;
  readonly stacked: boolean;
  readonly tight: boolean;
}

export interface HackingBoardFitFrameScheduler {
  cancelAnimationFrame(handle: FrameHandle): void;
  requestAnimationFrame(callback: FrameRequestCallback): FrameHandle;
}

export interface HackingBoardFitResizeObserver {
  disconnect(): void;
  observe(target: Element): void;
}

export interface HackingBoardFitOptions {
  readonly completeRowCounts?: () => readonly number[];
  readonly frameScheduler?: HackingBoardFitFrameScheduler;
  readonly observerFactory?: (callback: ResizeObserverCallback) => HackingBoardFitResizeObserver | null;
}

export interface HackingBoardFitController {
  readonly fit: Readonly<HackingBoardFit> | null;
  dispose(): void;
  schedule(): void;
  setBoard(board: HTMLElement | null): void;
}

const defaultFrameScheduler: HackingBoardFitFrameScheduler = Object.freeze({
  cancelAnimationFrame: (handle: FrameHandle) => globalThis.cancelAnimationFrame(handle),
  requestAnimationFrame: (callback: FrameRequestCallback) => globalThis.requestAnimationFrame(callback),
});

function defaultObserverFactory(callback: ResizeObserverCallback): HackingBoardFitResizeObserver | null {
  return typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(callback);
}

function overflows(element: Element): boolean {
  const region = element as HTMLElement;
  return region.scrollHeight > region.clientHeight + 1 || region.scrollWidth > region.clientWidth + 1;
}

function contains(parent: Element, child: Element): boolean {
  const tolerance = 1;
  const parentBounds = parent.getBoundingClientRect();
  const childBounds = child.getBoundingClientRect();
  return childBounds.top >= parentBounds.top - tolerance &&
    childBounds.left >= parentBounds.left - tolerance &&
    childBounds.right <= parentBounds.right + tolerance &&
    childBounds.bottom <= parentBounds.bottom + tolerance;
}

function contentOverflows(board: HTMLElement): boolean {
  const columns = board.querySelector<HTMLElement>('.hack-columns');
  const log = board.querySelector<HTMLElement>('.hack-log');
  const logPanel = board.querySelector<HTMLElement>('.hack-log-panel');
  const input = board.querySelector<HTMLElement>('.hack-input-line');
  if (columns === null || log === null || logPanel === null || input === null) return false;

  const columnElements = [...columns.children];
  const rows = [...columns.querySelectorAll<HTMLElement>('.hack-row')];
  const logLines = [...log.children];
  const regions: Element[] = [board, columns, logPanel, log, input, ...columnElements, ...rows, ...logLines];
  if (regions.some(overflows)) return true;

  const containment: ReadonlyArray<readonly [Element, Element]> = [
    [board, columns],
    [board, logPanel],
    [logPanel, log],
    [logPanel, input],
    ...columnElements.map(column => [columns, column] as const),
    ...rows.map(row => [columns, row] as const),
    ...logLines.map(line => [log, line] as const),
  ];
  return containment.some(([parent, child]) => !contains(parent, child));
}

function rowsFitColumns(board: HTMLElement): boolean {
  const columns = board.querySelector<HTMLElement>('.hack-columns');
  if (columns === null) return true;
  const tolerance = 0.5;
  return [...columns.children].every(column => {
    const columnBounds = column.getBoundingClientRect();
    return [...column.querySelectorAll<HTMLElement>('.hack-row')].every(row => {
      const address = row.querySelector<HTMLElement>('.hack-addr');
      const cells = [...row.querySelectorAll<HTMLElement>('.hcell')];
      const last = cells.at(-1) ?? row.querySelector<HTMLElement>('.hack-cells');
      if (address === null || last === null) return true;
      const addressBounds = address.getBoundingClientRect();
      const lastBounds = last.getBoundingClientRect();
      const rowBounds = row.getBoundingClientRect();
      return addressBounds.left >= columnBounds.left - tolerance &&
        lastBounds.right <= columnBounds.right + tolerance &&
        rowBounds.top >= columnBounds.top - tolerance &&
        rowBounds.bottom <= columnBounds.bottom + tolerance;
    });
  });
}

function fitRowFont(board: HTMLElement): number | null {
  board.style.removeProperty('--hack-row-font');
  const columns = board.querySelector<HTMLElement>('.hack-columns');
  if (columns === null || columns.children.length === 0 || columns.querySelector('.hack-row') === null) return null;
  const baseSize = Number.parseFloat(getComputedStyle(board).fontSize);
  if (!Number.isFinite(baseSize) || baseSize <= 0) return null;

  const fitsAt = (size: number): boolean => {
    board.style.setProperty('--hack-row-font', `${size}px`);
    return rowsFitColumns(board) && !contentOverflows(board);
  };
  if (!fitsAt(baseSize)) return baseSize;

  let low = baseSize;
  const columnWidths = [...columns.children].map(column => column.getBoundingClientRect().width);
  let high = Math.max(baseSize * 2, Math.min(...columnWidths));
  for (let attempt = 0; attempt < 8 && fitsAt(high); attempt += 1) {
    low = high;
    high *= 2;
  }
  while (high - low > 0.25) {
    const candidate = (low + high) / 2;
    if (fitsAt(candidate)) low = candidate;
    else high = candidate;
  }
  board.style.setProperty('--hack-row-font', `${low}px`);
  return low;
}

function measureProbe(probe: HTMLElement): Readonly<HackingBoardFit> {
  probe.style.removeProperty('--hack-row-font');
  probe.classList.remove('hack-compact', 'hack-stacked', 'hack-tight');
  const preferStacked = probe.clientWidth <= 700 || probe.clientHeight <= 300;
  probe.classList.toggle('hack-stacked', preferStacked);
  probe.classList.toggle('hack-compact', preferStacked || contentOverflows(probe));
  if (!preferStacked && contentOverflows(probe)) probe.classList.add('hack-compact', 'hack-stacked');
  if (contentOverflows(probe)) probe.classList.add('hack-tight');
  return Object.freeze({
    compact: probe.classList.contains('hack-compact'),
    fontSize: fitRowFont(probe),
    stacked: probe.classList.contains('hack-stacked'),
    tight: probe.classList.contains('hack-tight'),
  });
}

function applyFit(board: HTMLElement, fit: Readonly<HackingBoardFit> | null): void {
  board.classList.toggle('hack-compact', fit?.compact ?? false);
  board.classList.toggle('hack-stacked', fit?.stacked ?? false);
  board.classList.toggle('hack-tight', fit?.tight ?? false);
  if (fit?.fontSize === null || fit?.fontSize === undefined) board.style.removeProperty('--hack-row-font');
  else board.style.setProperty('--hack-row-font', `${fit.fontSize}px`);
}

export function fitHackingBoard(
  board: HTMLElement,
  completeRowCounts?: readonly number[],
): Readonly<HackingBoardFit> | null {
  const bounds = board.getBoundingClientRect();
  if (bounds.width <= 0 || bounds.height <= 0) return null;
  const probe = board.cloneNode(true) as HTMLElement;
  probe.removeAttribute('id');
  probe.querySelectorAll('[id]').forEach(element => element.removeAttribute('id'));
  probe.inert = true;
  probe.setAttribute('aria-hidden', 'true');
  probe.dataset.hackingFitProbe = 'true';
  if (completeRowCounts !== undefined) {
    [...probe.querySelectorAll<HTMLElement>('.hack-col')].forEach((column, index) => {
      const expected = completeRowCounts[index] ?? 0;
      const sample = column.querySelector<HTMLElement>('.hack-row');
      if (sample === null) return;
      while (column.querySelectorAll(':scope > .hack-row').length < expected) {
        column.append(sample.cloneNode(true));
      }
    });
  }
  Object.assign(probe.style, {
    height: `${bounds.height}px`,
    left: '-10000px',
    margin: '0',
    pointerEvents: 'none',
    position: 'fixed',
    top: '0',
    visibility: 'hidden',
    width: `${bounds.width}px`,
    zIndex: '-1',
  });
  document.body.append(probe);
  try {
    const fit = measureProbe(probe);
    applyFit(board, fit);
    return fit;
  } finally {
    probe.remove();
  }
}

export function createHackingBoardFitController(
  options: HackingBoardFitOptions = {},
  onFit?: (fit: Readonly<HackingBoardFit> | null) => void,
): HackingBoardFitController {
  const frames = options.frameScheduler ?? defaultFrameScheduler;
  const observerFactory = options.observerFactory ?? defaultObserverFactory;
  let board: HTMLElement | null = null;
  let disposed = false;
  let fit: Readonly<HackingBoardFit> | null = null;
  let frame: FrameHandle | null = null;
  let observer: HackingBoardFitResizeObserver | null = null;

  const measure = (): void => {
    frame = null;
    if (disposed || board === null) return;
    fit = fitHackingBoard(board, options.completeRowCounts?.());
    onFit?.(fit);
  };
  const schedule = (): void => {
    if (disposed || board === null) return;
    if (frame !== null) frames.cancelAnimationFrame(frame);
    frame = frames.requestAnimationFrame(measure);
  };

  return Object.freeze({
    get fit() { return fit; },
    dispose(): void {
      if (disposed) return;
      disposed = true;
      observer?.disconnect();
      observer = null;
      if (frame !== null) frames.cancelAnimationFrame(frame);
      frame = null;
      if (board !== null) applyFit(board, null);
      board = null;
      fit = null;
    },
    schedule,
    setBoard(next: HTMLElement | null): void {
      if (disposed || next === board) return;
      observer?.disconnect();
      observer = null;
      if (frame !== null) frames.cancelAnimationFrame(frame);
      frame = null;
      if (board !== null) applyFit(board, null);
      board = next;
      fit = null;
      if (board === null) return;
      observer = observerFactory(schedule);
      observer?.observe(board);
      schedule();
    },
  });
}

export function useHackingBoardFit(board: Ref<HTMLElement | null>, options: HackingBoardFitOptions = {}): HackingBoardFitController {
  const controller = createHackingBoardFitController(options);
  const stop = watch(board, next => controller.setBoard(next), { immediate: true });
  const dispose = (): void => {
    stop();
    controller.dispose();
  };
  onScopeDispose(dispose, true);
  return Object.freeze({
    get fit() { return controller.fit; },
    dispose,
    schedule: controller.schedule,
    setBoard: controller.setBoard,
  });
}
