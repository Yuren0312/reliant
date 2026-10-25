import { test, expect } from '@playwright/test';
import { devHarnessPresent } from './devHarness';

// The fixture harness runs the product viewer and URL router, not a screenshot-only replica.
test('dashboard viewer state survives share URL, reload and browser history', async ({ page }) => {
  test.setTimeout(60_000); // Three full dev-bundle navigations plus reload on shared machines.
  await page.goto('/forge-dashboards-preview?page=dashboard&dash=workspace/checkout-latency');
  const present = await devHarnessPresent(page, '/forge-dashboards-preview', page.getByTestId('dashboard-view'));
  test.skip(!present, 'dev-build-only harness');
  const markers = page.getByRole('checkbox', { name: 'Release markers' });
  const bands = page.getByRole('checkbox', { name: 'Alert bands' });
  await markers.uncheck();
  await expect(page).toHaveURL(/releases=false/);
  await bands.uncheck();
  await expect(page).toHaveURL(/alerts=false/);
  await page.getByTestId('dashboard-metrics').click();
  await page.getByRole('menuitemcheckbox', { name: 'p95 latency by route' }).click();
  await expect(page.getByTestId('dashboard-panel-latency-by-route')).toHaveCount(0);
  const shared = page.url();
  await page.reload();
  await expect(markers).not.toBeChecked();
  await expect(bands).not.toBeChecked();
  await expect(page.getByTestId('dashboard-panel-latency-by-route')).toHaveCount(0);
  await page.goBack();
  await expect(page.getByTestId('dashboard-panel-latency-by-route')).toBeVisible();
  await page.goBack();
  await expect(bands).toBeChecked();
  await page.goForward();
  await expect(bands).not.toBeChecked();
  await page.goto(shared);
  await expect(page.getByTestId('dashboard-panel-latency-by-route')).toHaveCount(0);
  await expect(page.getByTestId('dashboard-header')).toContainText('v4');
});
