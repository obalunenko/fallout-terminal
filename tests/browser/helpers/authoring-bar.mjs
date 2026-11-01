import { expect } from '@playwright/test';

// Journey helpers for the shared authoring bar (contracts/overseer-ui.md).

export async function saveStatus(page) {
  const status = page.locator('#saveStatus');
  return {
    text: (await status.textContent())?.trim() ?? '',
    state: await status.getAttribute('data-state'),
    unappliedCount: Number(await status.getAttribute('data-unapplied-count') ?? 0),
  };
}

export async function expectSaved(page) {
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-state', 'saved');
}

export async function undo(page) {
  await page.locator('#btnUndo').click();
  await expectSaved(page);
}

export async function redo(page) {
  await page.locator('#btnRedo').click();
  await expectSaved(page);
}

export async function switchTab(page, name) {
  await page.getByRole('tab', { name, exact: true }).click();
  await expect(page.getByRole('tab', { name, exact: true })).toHaveAttribute('aria-selected', 'true');
}

// The reason a blocked control shows through aria-describedby.
export async function blockedReason(control) {
  await expect(control).toHaveAttribute('aria-disabled', 'true');
  const id = await control.getAttribute('aria-describedby');
  return (await control.page().locator(`#${id}`).textContent())?.trim() ?? '';
}
