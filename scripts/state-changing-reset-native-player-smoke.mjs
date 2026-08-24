#!/usr/bin/env node

import fs from 'node:fs/promises';
import process from 'node:process';
import { chromium } from '../tests/browser/node_modules/playwright/index.mjs';

const [, , mode, baseURL, readyPath, triggerPath, resultPath] = process.argv;
const timeoutMilliseconds = 20_000;
const convergenceMilliseconds = 1_000;

function fail(message) {
  throw new Error(`native player smoke: ${message}`);
}

async function waitFor(description, condition, timeout = timeoutMilliseconds) {
  const deadline = Date.now() + timeout;
  let lastError = null;
  while (Date.now() <= deadline) {
    try {
      if (await condition()) return;
    } catch (error) {
      lastError = error;
    }
    await new Promise(resolve => setTimeout(resolve, 50));
  }
  const detail = lastError instanceof Error ? `: ${lastError.message}` : '';
  fail(`timed out waiting for ${description}${detail}`);
}

async function markerExists(path) {
  try {
    await fs.access(path);
    return true;
  } catch {
    return false;
  }
}

async function openPlayer(browser) {
  const context = await browser.newContext();
  await context.addInitScript(() => {
    localStorage.removeItem('fallout-terminal.player-token');
    HTMLMediaElement.prototype.play = () => Promise.resolve();
  });
  const page = await context.newPage();
  await page.goto(baseURL);
  await waitFor('player ConnectRPC subscription', () => page.locator('#connOverlay').evaluate(element => element.classList.contains('hidden')));
  return { context, page };
}

async function selectFirstAvailable(page) {
  const option = page.locator('#characterOptions button:not([disabled])').first();
  await waitFor('an available character', () => option.isVisible());
  await option.click();
  await waitFor('character selection acknowledgement', async () =>
    await page.locator('#characterSelect').getAttribute('aria-busy') !== 'true');
}

async function textVisible(page, selector, text) {
  return page.locator(selector).evaluateAll((elements, expected) => elements.some(element => {
    const style = getComputedStyle(element);
    return !element.hidden && style.display !== 'none' && style.visibility !== 'hidden' &&
      element.getClientRects().length > 0 && (element.textContent || '').includes(expected);
  }), text);
}

function rowWithText(page, text) {
  return page.locator('.term-row').filter({ hasText: text }).first();
}

async function runtimeRevision(page) {
  const raw = await page.locator('#screen').getAttribute('data-runtime-revision');
  const revision = Number(raw);
  if (!Number.isSafeInteger(revision) || revision <= 0) fail(`invalid runtime revision ${JSON.stringify(raw)}`);
  return revision;
}

async function assertRoles(players) {
  await waitFor('active controller role', () => textVisible(players[0].page, '#roleBadge', 'АКТИВЕН'));
  await waitFor('observer role', () => textVisible(players[1].page, '#roleBadge', 'НАБЛЮДАТЕЛЬ'));
}

async function enterAccessFolder(players) {
  const controller = players[0].page;
  await waitFor('access folder', () => textVisible(controller, '.term-row', 'УПРАВЛЕНИЕ ДОСТУПОМ'));
  await controller.keyboard.press('Shift');
  await rowWithText(controller, 'УПРАВЛЕНИЕ ДОСТУПОМ').click();
  try {
    await Promise.all(players.map(player =>
      waitFor('access folder projection', async () =>
        await textVisible(player.page, '.term-row', 'Открыть двери') ||
        await textVisible(player.page, '.term-row', 'Гермодвери открыты'))));
  } catch (error) {
    for (const [index, player] of players.entries()) {
      const diagnostics = await player.page.evaluate(() => ({
        role: document.querySelector('#roleBadge')?.textContent,
        notice: document.querySelector('#playerNotice')?.textContent,
        rows: document.querySelector('#termList')?.textContent,
        termListHidden: document.querySelector('#termList')?.hidden,
        termEntryHidden: document.querySelector('#termEntry')?.hidden,
        entryBody: document.querySelector('#entryBody')?.textContent,
        idleHidden: document.querySelector('#termIdle')?.hidden,
        runtimeRevision: document.querySelector('#screen')?.dataset.runtimeRevision,
        screenClasses: document.querySelector('#screen')?.className,
      }));
      console.error(`player ${index} diagnostics: ${JSON.stringify(diagnostics)}`);
    }
    throw error;
  }
}

async function assertInitialProjection(players) {
  await Promise.all(players.map(async player => {
    await waitFor('initial command names', async () =>
      await textVisible(player.page, '.term-row', 'Открыть двери') &&
      await textVisible(player.page, '.term-row', 'Отключить тревогу'));
    if (await rowWithText(player.page, 'Гермодвери открыты').count() !== 0) {
      fail('stale completed door name remains in player DOM');
    }
    if (await rowWithText(player.page, 'Тревога отключена').count() !== 0) {
      fail('stale completed alarm name remains in player DOM');
    }
    if (await player.page.locator('#termEntry').isVisible()) fail('stale completed result surface remains visible');
  }));
}

async function openJourney(browser) {
  const players = [await openPlayer(browser), await openPlayer(browser)];
  for (const player of players) await selectFirstAvailable(player.page);
  await assertRoles(players);
  await enterAccessFolder(players);
  return players;
}

async function runResetObservation(browser) {
  const players = await openJourney(browser);
  const controller = players[0].page;
  await waitFor('completed command before reset', () => textVisible(controller, '.term-row', 'Гермодвери открыты'));
  await rowWithText(controller, 'Гермодвери открыты').click();
  try {
    await Promise.all(players.map(player => waitFor('completed result before reset', () =>
      textVisible(player.page, '#entryBody', 'Гермодвери были открыты'))));
  } catch (error) {
    for (const [index, player] of players.entries()) {
      const diagnostics = await player.page.evaluate(() => ({
        role: document.querySelector('#roleBadge')?.textContent,
        notice: document.querySelector('#playerNotice')?.textContent,
        rows: document.querySelector('#termList')?.textContent,
        termListHidden: document.querySelector('#termList')?.hidden,
        termEntryHidden: document.querySelector('#termEntry')?.hidden,
        entryBody: document.querySelector('#entryBody')?.textContent,
        runtimeRevision: document.querySelector('#screen')?.dataset.runtimeRevision,
      }));
      console.error(`completed result player ${index} diagnostics: ${JSON.stringify(diagnostics)}`);
    }
    throw error;
  }

  const beforeRevisions = await Promise.all(players.map(player => runtimeRevision(player.page)));
  if (beforeRevisions[0] !== beforeRevisions[1]) fail(`players started on different revisions: ${beforeRevisions.join(',')}`);
  await fs.writeFile(readyPath, JSON.stringify({ beforeRevision: beforeRevisions[0] }));
  await waitFor('native reset trigger', () => markerExists(triggerPath));

  const convergenceStartedAt = Date.now();
  await assertInitialProjection(players);
  const convergenceElapsedMilliseconds = Date.now() - convergenceStartedAt;
  if (convergenceElapsedMilliseconds > convergenceMilliseconds) {
    fail(`player convergence took ${convergenceElapsedMilliseconds}ms (limit ${convergenceMilliseconds}ms)`);
  }
  const afterRevisions = await Promise.all(players.map(player => runtimeRevision(player.page)));
  if (afterRevisions[0] !== afterRevisions[1] || afterRevisions[0] <= beforeRevisions[0]) {
    fail(`runtime revision did not advance consistently: ${beforeRevisions.join(',')} -> ${afterRevisions.join(',')}`);
  }

  await controller.locator('#backBtn').click();
  await Promise.all(players.map(player => waitFor('shared navigation after reset', () =>
    textVisible(player.page, '.term-row', 'УПРАВЛЕНИЕ ДОСТУПОМ'))));
  await enterAccessFolder(players);
  await assertInitialProjection(players);

  await fs.writeFile(resultPath, JSON.stringify({
    mode,
    beforeRevision: beforeRevisions[0],
    afterRevision: afterRevisions[0],
    convergenceElapsedMilliseconds,
    controller: 'INITIAL',
    observer: 'INITIAL',
    staleResultAfterNavigation: false,
  }));
  await Promise.all(players.map(player => player.context.close()));
}

async function runReopenObservation(browser) {
  const players = await openJourney(browser);
  await assertInitialProjection(players);
  const revisions = await Promise.all(players.map(player => runtimeRevision(player.page)));
  if (revisions[0] !== revisions[1]) fail(`reopened players received different revisions: ${revisions.join(',')}`);
  await fs.writeFile(readyPath, JSON.stringify({ revision: revisions[0] }));
  await fs.writeFile(resultPath, JSON.stringify({
    mode,
    runtimeRevision: revisions[0],
    controller: 'INITIAL',
    observer: 'INITIAL',
    reopened: true,
  }));
  await Promise.all(players.map(player => player.context.close()));
}

async function runPresentationObservation(browser) {
  const players = await openJourney(browser);
  const [controller, observer] = players.map(player => player.page);
  const targetText = 'Тревога отключена';
  const blockedText = 'Гермодвери открыты';

  const feedbackStartedAt = Date.now();
  await rowWithText(controller, targetText).hover();
  await waitFor('next-frame controller presentation feedback', async () =>
    (await controller.locator('.term-row.sel').textContent())?.includes(targetText), 100);
  const feedbackElapsedMilliseconds = Date.now() - feedbackStartedAt;

  const convergenceStartedAt = Date.now();
  await waitFor('observer presentation convergence', async () =>
    (await observer.locator('.term-row.sel').textContent())?.includes(targetText), convergenceMilliseconds);
  const convergenceElapsedMilliseconds = Date.now() - convergenceStartedAt;

  await rowWithText(observer, blockedText).hover();
  await observer.keyboard.press('ArrowUp');
  await new Promise(resolve => setTimeout(resolve, 150));
  const selected = await Promise.all(players.map(player => player.page.locator('.term-row.sel').textContent()));
  if (selected.some(value => !value?.includes(targetText))) {
    fail(`observer input changed packaged presentation: ${JSON.stringify(selected)}`);
  }

  await fs.writeFile(readyPath, JSON.stringify({ presentationReady: true }));
  await fs.writeFile(resultPath, JSON.stringify({
    mode,
    feedbackElapsedMilliseconds,
    convergenceElapsedMilliseconds,
    controller: targetText,
    observer: targetText,
    observerInputSuppressed: true,
  }));
  await Promise.all(players.map(player => player.context.close()));
}

if (!['reset', 'reopen', 'presentation'].includes(mode) || !baseURL || !readyPath || !triggerPath || !resultPath) {
  fail('usage: state-changing-reset-native-player-smoke.mjs <reset|reopen|presentation> <base-url> <ready> <trigger> <result>');
}

const browser = await chromium.launch({ headless: true });
try {
  if (mode === 'reset') await runResetObservation(browser);
  else if (mode === 'reopen') await runReopenObservation(browser);
  else await runPresentationObservation(browser);
} finally {
  await browser.close();
}
