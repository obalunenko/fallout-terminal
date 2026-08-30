import playwright from '../../browser/node_modules/@playwright/test/index.js';

const { expect, test } = playwright;

test('focused browser exact target', () => {
  expect(1 + 1).toBe(2);
});
