import { expect, test } from 'playwright/test'

test('Connection Manager renders without a live OPC UA Server', async ({ page }) => {
  await page.goto('/?screenshot=connections-empty')

  await expect(page.getByRole('heading', { name: 'Connect to an OPC UA Server' })).toBeVisible()
  await expect(page.getByText('No Saved Connections yet.')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Discover Endpoints' })).toBeVisible()
})
