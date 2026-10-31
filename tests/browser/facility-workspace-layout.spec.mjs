import { expect, test } from '@playwright/test';

import { forceState, selectDevice } from './helpers/facility-play.mjs';
import { FIXTURE, demoSession, facilityState, openOverseer, seedDemo } from './helpers/overseer-forms.mjs';

// User Story 2: see the selected device first
// (specs/040-facility-play-controls/spec.md, contracts/overseer-ui.md).

const guide = page => page.locator('#facilityStoryGuide');
const guideToggle = page => page.locator('#facilityStoryGuideToggle');

async function reopen(page) {
  await page.reload();
  await openOverseer(page);
}

test('the guide is open without devices and collapses to one line on the first device (AS1, AS2, FR-019)', async ({ page, request }) => {
  const reset = await request.post(`${FIXTURE}/reset`, { data: { scenario: 'empty' } });
  expect(reset.status()).toBe(204);
  await openOverseer(page);
  await expect(guide(page)).toHaveAttribute('open', '');
  await expect(guideToggle(page)).toHaveText('СКРЫТЬ');

  await guide(page).getByRole('button', { name: 'СОЗДАТЬ УСТРОЙСТВО', exact: true }).click();
  const device = page.getByRole('dialog', { name: 'УСТРОЙСТВО ОБЪЕКТА', exact: true });
  await device.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Дверь в убежище');
  await device.getByLabel('ТИП', { exact: true }).selectOption('door');
  for (const [id, name] of [['closed', 'Заперта'], ['open', 'Открыта']]) {
    await device.getByRole('button', { name: 'ДОБАВИТЬ СОСТОЯНИЕ', exact: true }).click();
    const row = device.locator('.facility-state-row').last();
    await row.locator('.facility-state-id').fill(id);
    await row.locator('.facility-state-name').fill(name);
  }
  await device.getByLabel('НАЧАЛЬНОЕ СОСТОЯНИЕ', { exact: true }).selectOption('closed');
  await device.getByRole('button', { name: 'ГОТОВО', exact: true }).click();

  await expect.poll(async () => (await facilityState(request)).facility.devices.length).toBe(1);
  await expect(guide(page)).not.toHaveAttribute('open', '');
  await expect(page.locator('#facilityStoryGuideStep')).toHaveText('КАК СОБРАТЬ СЦЕНАРИЙ · ШАГ 1 ИЗ 5: ОБЪЕКТ');
  await expect(guideToggle(page)).toHaveText('ПОКАЗАТЬ');
  // Focus stays on a visible control when the guide collapses under it.
  await expect(page.locator('#facilityStoryGuideSummary')).toBeFocused();
});

test('the author\'s choice holds for the run and the rule returns after a restart (AS3, FR-020)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  await expect(guide(page)).not.toHaveAttribute('open', '');
  await page.locator('#facilityStoryGuideSummary').click();
  await expect(guide(page)).toHaveAttribute('open', '');
  await expect(guideToggle(page)).toHaveText('СКРЫТЬ');

  await page.getByRole('tab', { name: 'ТЕРМИНАЛЫ', exact: true }).click();
  await page.getByRole('tab', { name: 'ОБЪЕКТЫ', exact: true }).click();
  await selectDevice(page, 'vault-door-main');
  await selectDevice(page, 'reactor-main');
  await expect(guide(page)).toHaveAttribute('open', '');

  await reopen(page);
  await expect(guide(page)).not.toHaveAttribute('open', '');
  // Nothing about the guide is stored.
  expect(await page.evaluate(() => JSON.stringify({ ...localStorage }))).not.toMatch(/guide|story/i);
});

test('the device header comes first and names the device, its states, faults, and terminals (FR-017)', async ({ page, request }) => {
  await seedDemo(request);
  await forceState(request, 'ventilation-main', 'contaminated').catch(() => {});
  await openOverseer(page);
  await selectDevice(page, 'ventilation-main');
  const editor = page.locator('#facilitySelectionEditor');
  await expect(editor.locator(':scope > *').first()).toHaveId('facilityDeviceHeader');
  await expect(page.locator('#facilityDeviceHeaderName')).toHaveText('Контур вентиляции теплицы');
  await expect(page.locator('#facilityDeviceHeaderKind')).toHaveAttribute('data-kind', 'ventilation');
  await expect(page.locator('#facilityDeviceHeaderCurrent')).toHaveText('СЕЙЧАС: Загрязнён');
  await expect(page.locator('#facilityDeviceHeaderInitial')).toHaveText('В НАЧАЛЕ: Загрязнён');
  await expect(page.locator('#facilityDeviceHeaderFaults')).toHaveText(/^АКТИВНЫХ НЕИСПРАВНОСТЕЙ: \d+$/);
  await expect(page.locator('#facilityDeviceHeaderTerminals')).toHaveText(/^ТЕРМИНАЛЫ: [1-9]\d*$/);
  // The header, then the play card, then the actions.
  const order = await editor.evaluate(root => [...root.children].map(child => child.id).filter(Boolean));
  expect(order.slice(0, 3)).toEqual(['facilityDeviceHeader', 'facilityPlayCard', 'facilityDeviceActions']);
});

for (const viewport of [{ width: 1440, height: 900 }, { width: 1180, height: 760 }]) {
  test(`every demo device shows its header and first play control without scrolling at ${viewport.width}×${viewport.height} (AS4, FR-018, SC-003)`, async ({ page, request }) => {
    await page.setViewportSize(viewport);
    const demo = await demoSession();
    await seedDemo(request);
    await openOverseer(page);
    await expect(guide(page)).not.toHaveAttribute('open', '');
    for (const device of demo.facility.devices) {
      await selectDevice(page, device.id);
      const panel = page.locator('#facilityDetailPanel');
      await panel.evaluate(element => { element.scrollTop = 0; });
      const run = page.locator('.facility-play-transition-run').first();
      const control = await run.count() ? run : page.locator('#btnForceFacilityDeviceState');
      const boxes = {
        panel: await panel.boundingBox(),
        header: await page.locator('#facilityDeviceHeader').boundingBox(),
        control: await control.boundingBox(),
      };
      for (const name of ['header', 'control']) {
        const box = boxes[name];
        expect(box, `${device.id} ${name}`).not.toBeNull();
        expect(box.y + box.height, `${device.id} ${name} bottom`).toBeLessThanOrEqual(Math.min(viewport.height, boxes.panel.y + boxes.panel.height));
        expect(box.y, `${device.id} ${name} top`).toBeGreaterThanOrEqual(boxes.panel.y);
        expect(box.x + box.width, `${device.id} ${name} right`).toBeLessThanOrEqual(viewport.width);
      }
      expect(boxes.control.y, `${device.id} header and control overlap`).toBeGreaterThanOrEqual(boxes.header.y + boxes.header.height);
    }
  });
}

for (const width of [900, 1180, 1440]) {
  test(`the open guide's step bar never covers other content at ${width} wide (AS5, FR-021)`, async ({ page, request }) => {
    await page.setViewportSize({ width, height: 760 });
    await seedDemo(request);
    await openOverseer(page);
    await page.locator('#facilityStoryGuideSummary').click();
    await selectDevice(page, 'vault-door-main');
    const steps = page.locator('.facility-story-steps');
    await expect(steps).toHaveCSS('position', 'static');
    const panel = page.locator('#facilityDetailPanel');
    const maxScroll = await panel.evaluate(element => element.scrollHeight - element.clientHeight);
    for (const fraction of [0, 0.25, 0.5, 0.75, 1]) {
      await panel.evaluate((element, top) => { element.scrollTop = top; }, Math.round(maxScroll * fraction));
      const overlaps = await steps.evaluate(nav => {
        const rect = nav.getBoundingClientRect();
        const panelRect = nav.closest('#facilityDetailPanel').getBoundingClientRect();
        if (rect.bottom <= panelRect.top || rect.top >= panelRect.bottom) return [];
        const covered = [];
        for (const element of nav.closest('#facilityDetailPanel').querySelectorAll('button, h2, h3, p, input, select')) {
          if (nav.contains(element) || element.contains(nav) || !element.getClientRects().length) continue;
          const other = element.getBoundingClientRect();
          const width = Math.min(rect.right, other.right) - Math.max(rect.left, other.left);
          const height = Math.min(rect.bottom, other.bottom) - Math.max(rect.top, other.top);
          if (width > 1 && height > 1) covered.push(element.id || element.textContent.trim().slice(0, 30));
        }
        return covered;
      });
      expect(overlaps, `scroll ${fraction}`).toEqual([]);
    }
  });
}
