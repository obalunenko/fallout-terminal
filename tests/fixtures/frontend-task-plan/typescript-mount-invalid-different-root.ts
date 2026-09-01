interface PlayerPorts { close(): void }
declare function createApp(value: unknown): { mount(root: HTMLElement): void };
declare const App: unknown;
function mountPlayerApp(root: HTMLElement, ports: PlayerPorts): void {
  void ports;
  const other = document.createElement('div');
  createApp(App).mount(other);
  void root;
}
