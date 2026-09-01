import { expect, test } from '@playwright/test';

const overseerAppModuleURL = 'http://127.0.0.1:34121/@fs' + new URL('./fixtures/overseer-app.ts', import.meta.url).pathname;
const expectedAssertion = 'production Overseer root entry CSP assets and public selectors are exact';

test.use({ bypassCSP: true });

test(expectedAssertion, async ({ page }) => {
  const browserErrors = [];
  page.on('console', message => {
    if (message.type() === 'error') browserErrors.push(message.text());
  });
  page.on('pageerror', error => browserErrors.push(error.message));

  await page.goto('/__fixture/state-changing-command-authoring');
  await page.evaluate(url => import(url), overseerAppModuleURL + '?production-promotion');

  await expect(page.locator('#overseerApp')).toHaveCount(1);
  await expect(page.locator('#startScreen')).toHaveCount(1);
  await expect(page.locator('#mainLayout')).toHaveCount(1);
  await expect(page.locator('#runtimeHeader')).toHaveCount(1);
  await expect(page.locator('#coordinationPanel')).toHaveCount(1);
  await expect(page.locator('#termList')).toHaveCount(1);
  await expect(page.locator('#terminalEditorVueLeaf')).toHaveCount(1);
  await expect(page.locator('#terminalTreeVueLeaf')).toHaveCount(1);
  await expect(page.locator('#nodeEditorVueLeaf')).toHaveCount(1);
  await expect(page.locator('#publicAccessVueLeaf')).toHaveCount(1);
  await expect(page.locator('#createTerminalDialog')).toHaveCount(1);
  await expect(page.locator('#terminalGroupDraftDialog')).toHaveCount(1);
  await expect(page.locator('#terminalGroupImpactDialog')).toHaveCount(1);
  expect(browserErrors, browserErrors.join('\n')).toEqual([]);
});
