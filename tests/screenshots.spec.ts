import { test } from './fixtures';

/** Visual review screenshots — single page now. */
test.describe('yfinance visual review', () => {
  test('instruments page', async ({ gotoPage, page }) => {
    await gotoPage('/instruments');
    await page.waitForLoadState('networkidle');
    await page.screenshot({
      path: 'test-results/screenshots/instruments.png',
      fullPage: true,
    });
  });
});
