import playwright from '../../browser/node_modules/@playwright/test/index.js';

const { expect, test } = playwright;

test('first shared focused suffix', () => {
  expect(true).toBe(true);
});

test('second shared focused suffix', () => {
  expect(false).toBe(false);
});
