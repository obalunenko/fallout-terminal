import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const focusModulePath = fileURLToPath(new URL(
  '../../frontend/overseer/src/composables/useDialogFocus.ts',
  import.meta.url,
));
const focusModuleURL = `http://127.0.0.1:34120/@fs${focusModulePath}`;
const directiveModulePath = fileURLToPath(new URL(
  '../../frontend/overseer/src/directives/dialog-focus.ts',
  import.meta.url,
));
const directiveModuleURL = `http://127.0.0.1:34120/@fs${directiveModulePath}`;

test('disconnected opener is not focused after unmount', async ({ page }) => {
  if (!existsSync(focusModulePath)) {
    process.stderr.write('AssertionError: disconnected opener is not focused after unmount\n');
    throw new Error('Vue-owned dialog focus lifecycle is not implemented');
  }

  await page.goto('http://127.0.0.1:34120/');
  const observation = await page.evaluate(async ({ directiveURL, moduleURL }) => {
    const { createDialogFocusController } = await import(moduleURL);
    const { dialogFocus } = await import(directiveURL);

    const connectedOpener = document.createElement('button');
    document.body.append(connectedOpener);
    let connectedRestoreCalls = 0;
    connectedOpener.focus = () => { connectedRestoreCalls += 1; };
    const connectedController = createDialogFocusController();
    connectedController.capture(connectedOpener);
    connectedController.restore();
    await Promise.resolve();

    const disconnectedOpener = document.createElement('button');
    document.body.append(disconnectedOpener);
    let disconnectedRestoreCalls = 0;
    disconnectedOpener.focus = () => { disconnectedRestoreCalls += 1; };
    const disconnectedController = createDialogFocusController();
    disconnectedController.capture(disconnectedOpener);
    disconnectedOpener.remove();
    disconnectedController.restore();
    disconnectedController.dispose();

    const dialog = document.createElement('dialog');
    const initialTarget = document.createElement('button');
    dialog.append(initialTarget);
    document.body.append(dialog);
    let initialFocusCalls = 0;
    let cancelCalls = 0;
    initialTarget.focus = () => { initialFocusCalls += 1; };
    const binding = {
      value: {
        active: true,
        initialFocus: () => initialTarget,
        onCancel: () => { cancelCalls += 1; },
        opener: connectedOpener,
      },
    };
    dialogFocus.mounted(dialog, binding);
    await Promise.resolve();
    const cancelEvent = new Event('cancel', { cancelable: true });
    dialog.dispatchEvent(cancelEvent);
    dialogFocus.beforeUnmount(dialog);
    dialog.dispatchEvent(new Event('cancel', { cancelable: true }));
    await Promise.resolve();

    return {
      cancelCalls,
      cancelPrevented: cancelEvent.defaultPrevented,
      connectedRestoreCalls,
      disconnectedRestoreCalls,
      initialFocusCalls,
    };
  }, { directiveURL: directiveModuleURL, moduleURL: focusModuleURL });

  expect(observation).toEqual({
    cancelCalls: 1,
    cancelPrevented: true,
    connectedRestoreCalls: 1,
    disconnectedRestoreCalls: 0,
    initialFocusCalls: 1,
  });
});
