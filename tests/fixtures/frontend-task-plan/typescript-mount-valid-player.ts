interface PlayerPorts { close(): void }
declare function createApp(value: unknown): { mount(root: HTMLElement): void };
declare const App: unknown;
function mountPlayerApp(root: HTMLElement, ports: PlayerPorts): void {
  void ports;
  createApp(App).mount(root);
}
