interface PlayerPorts { close(): void }
declare function createApp(value: unknown): { mount(root: HTMLElement): void };
declare const App: unknown;
function unrelated(ports: PlayerPorts): void { void ports; }
function mountPlayerApp(root: HTMLElement, ports: PlayerPorts): void {
  void ports;
  createApp(App).mount(root);
}
