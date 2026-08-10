import { expect, test } from 'playwright/test'

test('Search is a dedicated sidebar screen and preserves shared inspection state', async ({ page }) => {
  await page.goto('/?screenshot=hero')

  const sidebar = page.locator('aside')
  const labels = await sidebar.locator('nav button').allTextContents()
  expect(labels.findIndex(label => label.includes('Search'))).toBe(labels.findIndex(label => label.includes('Address Space')) + 1)

  await sidebar.getByRole('button', { name: 'Search', exact: true }).click()

  await expect(page.getByRole('heading', { name: 'Address Space Search' })).toBeVisible()
  await expect(page.getByLabel('Search Address Space', { exact: true })).toBeFocused()
  await expect(page.getByText('3 Search Results found in browsed Address Space metadata.')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Filler Temperature', level: 2 })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Objects', level: 2 })).toHaveCount(0)

  await sidebar.getByRole('button', { name: /Address Space$/ }).click()
  await expect(page.getByRole('heading', { name: 'Objects', level: 2 })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Filler Temperature', level: 2 })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Address Space Search' })).toHaveCount(0)
})

test('the header Search field opens Search and shares its query', async ({ page }) => {
  await page.addInitScript(() => {
    const testWindow = window as typeof window & { go: unknown }
    testWindow.go = { main: { App: {
      SearchAddressSpace: (query: string) => Promise.resolve({ query, results: [], status: `Search complete for ${query}` })
    } } }
  })
  await page.goto('/?screenshot=hero')

  const headerSearch = page.getByLabel('Search Address Space from header')
  await headerSearch.focus()

  await expect(page.getByRole('heading', { name: 'Address Space Search' })).toBeVisible()
  await expect(headerSearch).toBeFocused()
  await expect(page.getByLabel('Search Address Space', { exact: true })).toHaveValue('filler')

  await headerSearch.fill('pressure')
  await expect(page.getByLabel('Search Address Space', { exact: true })).toHaveValue('pressure')
  await expect(page.getByRole('status')).toContainText('Search complete for pressure')
})

test('Search remains navigable but its inputs are disabled while disconnected', async ({ page }) => {
  await page.goto('/?screenshot=search-disconnected')

  await expect(page.getByRole('heading', { name: 'Address Space Search' })).toBeVisible()
  await expect(page.getByLabel('Search Address Space from header')).toBeDisabled()
  await expect(page.getByLabel('Search Address Space', { exact: true })).toBeDisabled()
  await expect(page.getByText('Connect to an OPC UA Server to search browsed Address Space metadata.')).toBeVisible()
  await expect(page.locator('aside').getByRole('button', { name: 'Search', exact: true })).toBeEnabled()
})

test('an Object Search Result explains that no inspection panel is available', async ({ page }) => {
  await page.goto('/?screenshot=search-object')

  await expect(page.getByRole('heading', { name: 'Address Space Search' })).toBeVisible()
  await expect(page.getByText('Filler Station', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('Object Nodes do not provide Variable Node Inspection or Method Call details.')).toBeVisible()
})
