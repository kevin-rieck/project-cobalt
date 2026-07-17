import { expect, test, type Page } from 'playwright/test'

async function openVariableNodeInspection(page: Page, state: string) {
  await page.goto(`/?screenshot=${state}`)
  await expect(page.getByRole('heading', { name: 'Write value' })).toBeVisible()
  return page.getByRole('button', { name: 'Write value', exact: true })
}

test('Read-Only Mode explains why Variable Node Write is unavailable', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-read-only')

  await expect(writeValue).toBeDisabled()
  await expect(page.getByText('Read-Only Mode is active.')).toBeVisible()
})

test('failed metadata explains that write availability cannot be determined', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-metadata-failed')

  await expect(writeValue).toBeDisabled()
  await expect(page.getByText('Write availability cannot be determined until metadata loads.')).toBeVisible()
})

test('a stale Live Value explains why Variable Node Write is unavailable', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-live-value-stale')

  await expect(writeValue).toBeDisabled()
  await expect(page.getByText('Current Live Value is stale or unavailable.')).toBeVisible()
})

test('an unsupported data type gives a data-type-specific disabled reason', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-data-type-unsupported')

  await expect(writeValue).toBeDisabled()
  await expect(page.getByText('Data type DateTime is not supported for Variable Node Write.')).toBeVisible()
})

test('a non-scalar ValueRank gives the scalar-only disabled reason', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-value-rank-non-scalar')

  await expect(writeValue).toBeDisabled()
  await expect(page.getByText('Only scalar Variable Node Writes are supported; ValueRank is OneDimension.')).toBeVisible()
})

test('non-writable metadata shows the effective write availability wording', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-metadata-non-writable')

  await expect(writeValue).toBeDisabled()
  await expect(page.getByRole('listitem').filter({ hasText: /^Read-only in this session$/ })).toBeVisible()
})
