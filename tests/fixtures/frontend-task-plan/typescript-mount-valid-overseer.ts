interface DesktopPort {
  close(): void;
}

declare function createApp(value: unknown): { mount(root: HTMLElement): void };
declare const App: unknown;

function mountOverseerApp(
  root: HTMLElement,
  port: DesktopPort,
): void {
  void port;
  createApp(App).mount(root);
}
