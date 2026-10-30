import { readFile } from 'node:fs/promises';

import { expect } from '@playwright/test';

export const FIXTURE = '/__fixture/facility-authoring';
const DEMO_URL = new URL('../../../sessions/demo.json', import.meta.url);

export async function facilityState(request) {
  const response = await request.get(`${FIXTURE}/state`);
  expect(response.ok()).toBe(true);
  return response.json();
}

export async function resetFixture(request, scenario = 'operations') {
  const response = await request.post(`${FIXTURE}/reset`, { data: { scenario } });
  expect(response.status()).toBe(204);
}

export async function fixtureSession(request) {
  const response = await request.get(`${FIXTURE}/session`);
  expect(response.ok()).toBe(true);
  return response.json();
}

// Replaces the fixture session through the same save path the Overseer uses.
export async function seedSession(request, session) {
  const before = await facilityState(request);
  const response = await request.post(`${FIXTURE}/save`, {
    data: {
      session,
      expectedSessionRevision: before.sessionRevision,
      expectedFacilityRevision: before.facility.revision,
      correlationId: `seed-${Date.now()}`,
    },
  });
  expect(response.ok()).toBe(true);
  expect(await response.json()).toMatchObject({ ok: true });
  return facilityState(request);
}

export async function demoSession() {
  return JSON.parse(await readFile(DEMO_URL, 'utf8'));
}

export async function seedDemo(request) {
  await resetFixture(request);
  const demo = await demoSession();
  await seedSession(request, demo);
  return demo;
}

export async function openOverseer(page, tab = 'ОБЪЕКТЫ') {
  await page.goto(`${FIXTURE}/overseer`);
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ', exact: true }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
  if (tab) await page.getByRole('tab', { name: tab, exact: true }).click();
}

// Asserts that every visible label, legend, and help text in scope describes a visible control.
export async function expectNoOrphanLabels(scope, context = '') {
  const orphans = await scope.evaluate(root => {
    const CONTROL = 'input, select, textarea, button, output';
    const HELP = '.facility-section-help, .public-access-hint, .command-mode-hint';
    const visible = element => Boolean(element) && element.getClientRects().length > 0
      && getComputedStyle(element).visibility !== 'hidden';
    const text = element => element.textContent.trim().replace(/\s+/g, ' ').slice(0, 60);
    const isControl = element => Boolean(element) && element.matches(CONTROL);
    const problems = [];

    // Only real labels name a control; .field-label-inline spans may caption a summary row instead.
    for (const label of root.querySelectorAll('label')) {
      if (!visible(label)) continue;
      let control = null;
      if (label.htmlFor) control = document.getElementById(label.htmlFor);
      else control = label.querySelector(CONTROL)
        || (isControl(label.nextElementSibling) ? label.nextElementSibling : null);
      if (label.htmlFor && !control) problems.push(`label without target: ${text(label)}`);
      else if (control && !visible(control)) problems.push(`label of hidden control: ${text(label)}`);
    }

    for (const legend of root.querySelectorAll('legend')) {
      if (!visible(legend)) continue;
      const controls = [...legend.parentElement.querySelectorAll(CONTROL)];
      if (!controls.some(visible)) problems.push(`legend without visible control: ${text(legend)}`);
    }

    const describers = new Map();
    for (const control of root.querySelectorAll('[aria-describedby]')) {
      for (const id of control.getAttribute('aria-describedby').split(/\s+/).filter(Boolean)) {
        if (!describers.has(id)) describers.set(id, []);
        describers.get(id).push(control);
      }
    }
    for (const [id, controls] of describers) {
      const help = document.getElementById(id);
      if (!help || !root.contains(help) || !visible(help) || !text(help)) continue;
      if (help.matches('[role="alert"], [role="status"]')) continue;
      if (!controls.some(visible)) problems.push(`description of hidden controls: ${text(help)}`);
    }

    for (const help of root.querySelectorAll(HELP)) {
      if (!visible(help)) continue;
      const previous = help.previousElementSibling;
      if (isControl(previous) && !visible(previous)) problems.push(`help after hidden control: ${text(help)}`);
    }
    return problems;
  });
  expect(orphans, `orphan labels ${context}`.trim()).toEqual([]);
}
