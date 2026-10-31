import { expect, test } from '@playwright/test';

import {
  feedEntries, forceState, openPlayerTable, playCard, PLAYER_FIXTURE, selectDevice, setConditionActive,
} from './helpers/facility-play.mjs';
import { FIXTURE, facilityState, openOverseer, seedDemo } from './helpers/overseer-forms.mjs';

// User Story 5: see what happened to a device without watching
// (specs/040-facility-play-controls/spec.md, contracts/overseer-ui.md).

const DOOR = 'vault-door-main';
const EVACUATION = 'n_cmd_open_evacuation';
const TERMINAL = 'Терминал смотрителя — Убежище 76';

// openDoorScene opens the door card on a demo copy where the evacuation
// command can run: grid online, door sealed, its controller fault off.
async function openDoorScene(page, request) {
  await seedDemo(request);
  const devices = (await facilityState(request)).facility.devices;
  for (const [id, stateID] of [['power-grid-main', 'online'], [DOOR, 'sealed'], ['evacuation-alarm', 'quiet']]) {
    if (devices.find(device => device.id === id).currentStateId !== stateID) await forceState(request, id, stateID);
  }
  // No fault may block the command unless a test switches one on.
  for (const fault of ['door-offline', 'grid-unpowered']) await setConditionActive(request, fault, false);
  await openOverseer(page);
  await selectDevice(page, DOOR);
  return playCard(page);
}

async function playerCommand(request, decision) {
  const response = await request.post(`${FIXTURE}/player-command`, {
    data: { terminalId: 't_demo1', commandId: EVACUATION, decision },
  });
  expect(response.ok()).toBe(true);
}

async function waitForFeedHead(page, kind) {
  await expect.poll(async () => (await feedEntries(page))[0]?.kind).toBe(kind);
}

test('a fresh run says nothing changed yet (AS4)', async ({ page, request }) => {
  const card = await openDoorScene(page, request);
  await expect(card.feed).toHaveCount(0);
  await expect(card.feedEmpty).toHaveText('В этом запуске изменений ещё не было.');
});

test('the feed lists every kind of change newest first and keeps five (AS1, AS2, FR-012)', async ({ page, request }) => {
  const card = await openDoorScene(page, request);

  await card.run('open').click();
  await waitForFeedHead(page, 'transition');
  await card.force.click();
  await page.locator('#facilityForceStateTarget').selectOption('sealed');
  await page.locator('#btnConfirmFacilityForceState').click();
  await waitForFeedHead(page, 'forced');
  await card.faultSwitch('door-offline').click();
  await waitForFeedHead(page, 'fault');
  await card.reset.click();
  await page.locator('#btnConfirmFacilityDeviceReset').click();
  await waitForFeedHead(page, 'reset');
  await playerCommand(request, 'approve');
  await waitForFeedHead(page, 'player-command');
  await playerCommand(request, 'reject');
  await waitForFeedHead(page, 'player-refusal');

  const entries = await feedEntries(page);
  expect(entries.map(entry => entry.kind)).toEqual(['player-refusal', 'player-command', 'reset', 'fault', 'forced']);
  expect(entries.map(entry => entry.sequence)).toEqual([...entries.map(entry => entry.sequence)].sort((a, b) => b - a));
  expect(entries.map(entry => entry.author)).toEqual(['terminal', 'terminal', 'overseer', 'overseer', 'overseer']);
  const time = '\\d\\d:\\d\\d · ';
  expect(entries[0].text).toMatch(new RegExp(`^${time}«Открыть выход и подать сигнал» · запрос игрока отклонён: Смотрителем · ${TERMINAL}$`));
  expect(entries[0].reason).toBe('declined');
  expect(entries[1].text).toMatch(new RegExp(`^${time}«Открыть выход и подать сигнал»: Заперта → Открыта · ${TERMINAL}$`));
  expect(entries[2].text).toMatch(new RegExp(`^${time}сброс к начальному: неисправности восстановлены · Смотритель$`));
  expect(entries[3].text).toMatch(new RegExp(`^${time}неисправность «Контроллер гермодвери не отвечает» активирована · Смотритель$`));
  expect(entries[4].text).toMatch(new RegExp(`^${time}принудительно: Открыта → Заперта · Смотритель$`));
  // The sixth change dropped the oldest entry, the card transition.
  expect(entries.some(entry => entry.kind === 'transition')).toBe(false);
  for (const entry of entries) expect(entry.text).not.toMatch(/пароль|password|взлом/i);
});

test('refusals name their reason (AS2, FR-013)', async ({ page, request }) => {
  const card = await openDoorScene(page, request);

  await card.faultSwitch('door-offline').click();
  await waitForFeedHead(page, 'fault');
  await playerCommand(request, 'approve');
  await expect.poll(async () => (await feedEntries(page))[0]?.reason).toBe('fault-blocked');
  expect((await feedEntries(page))[0].text).toContain('запрос игрока отклонён: мешает неисправность');

  await card.faultSwitch('door-offline').click();
  await waitForFeedHead(page, 'fault');
  await forceState(request, 'power-grid-main', 'offline');
  await playerCommand(request, 'approve');
  await expect.poll(async () => (await feedEntries(page))[0]?.reason).toBe('precondition-failed');
  expect((await feedEntries(page))[0].text).toContain('запрос игрока отклонён: предусловие не выполнено');
});

test('a new entry appears while the card is open, without moving focus (AS3, FR-014)', async ({ page, request }) => {
  const card = await openDoorScene(page, request);
  await card.force.focus();
  await playerCommand(request, 'approve');
  await waitForFeedHead(page, 'player-command');
  await expect(card.force).toBeFocused();
  await expect(page.locator('#facilityDeviceHeaderCurrent')).toHaveText('СЕЙЧАС: Открыта');
});

test('opening another session starts with an empty feed (FR-012)', async ({ page, request }) => {
  const card = await openDoorScene(page, request);
  await card.run('open').click();
  await waitForFeedHead(page, 'transition');
  await openDoorScene(page, request);
  await expect(card.feed).toHaveCount(0);
  await expect(card.feedEmpty).toBeVisible();
});

test('a pending player request withdrawn by a play change is fed as withdrawn (FR-006)', async ({ browser, request }) => {
  const table = await openPlayerTable(browser, request);
  try {
    const { overseer, player } = table;
    await selectDevice(overseer, 'device-security-door');
    await player.locator('.term-row', { hasText: 'OPEN SECURITY DOOR' }).click();
    const approval = overseer.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' });
    await expect(approval).toBeVisible();

    const before = await (await request.get(`${PLAYER_FIXTURE}/state`)).json();
    const response = await request.post(`${PLAYER_FIXTURE}/force-state`, {
      data: {
        deviceId: 'device-security-door', stateId: 'open',
        expectedFacilityRevision: before.facility.revision, correlationId: 'journey-withdraw',
      },
    });
    expect((await response.json()).ok).toBe(true);

    await expect(approval).toBeHidden();
    await expect.poll(async () => (await feedEntries(overseer)).map(entry => entry.kind))
      .toEqual(['player-refusal', 'forced']);
    const [withdrawn] = await feedEntries(overseer);
    expect(withdrawn.reason).toBe('withdrawn');
    expect(withdrawn.text).toContain('«OPEN SECURITY DOOR» · запрос игрока отклонён: отозван после действия Смотрителя');
  } finally {
    await table.close();
  }
});
