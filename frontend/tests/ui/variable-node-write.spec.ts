import { expect, test, type Page } from 'playwright/test'

async function openVariableNodeInspection(page: Page, state: string) {
  await page.goto(`/?screenshot=${state}`)
  await expect(page.getByRole('heading', { name: 'Write value' })).toBeVisible()
  return page.getByRole('button', { name: 'Write value', exact: true })
}

async function expectConfirmationDetail(page: Page, label: string, value: string) {
  const detail = page.getByRole('dialog').locator('dt', { hasText: label }).filter({ hasText: new RegExp(`^${label}$`) }).locator('..')
  await expect(detail.getByText(value, { exact: true })).toBeVisible()
}

async function stubVariableNodeWrite(page: Page, result: unknown, error = '') {
  await page.addInitScript(({ result, error }) => {
    const write = error
      ? () => Promise.reject(new Error(error))
      : () => Promise.resolve(result)
    ;(window as typeof window & { go: unknown }).go = { main: { App: { WriteVariableNodeValue: write } } }
  }, { result, error })
}

async function submitVariableNodeWrite(page: Page, state = 'write-confirmation', target = '84.2') {
  const writeValue = await openVariableNodeInspection(page, state)
  await page.getByLabel('Target Value').fill(target)
  await writeValue.click()
  await page.getByRole('button', { name: 'Confirm write' }).click()
}

test('Variable Node Write confirmation requires deliberate review', async ({ page }) => {
  await page.addInitScript(() => {
    const testWindow = window as typeof window & { writeCalls: number; go: unknown }
    testWindow.writeCalls = 0
    testWindow.go = { main: { App: { WriteVariableNodeValue: () => { testWindow.writeCalls += 1 } } } }
  })
  const writeValue = await openVariableNodeInspection(page, 'write-confirmation')
  const targetValue = page.getByLabel('Target Value')
  const confirmationHeading = page.getByRole('heading', { name: 'This changes the OPC UA Server' })

  await expect(writeValue).toBeDisabled()
  await targetValue.fill('84.2')
  await expect(writeValue).toBeEnabled()

  await targetValue.press('Enter')
  await expect(confirmationHeading).toBeHidden()
  await expect(targetValue).toHaveValue('84.2')
  await expect(writeValue).toHaveText('Write value')
  await expect.poll(() => page.evaluate(() => (window as typeof window & { writeCalls: number }).writeCalls)).toBe(0)

  await writeValue.click()
  await expect(confirmationHeading).toBeVisible()
  await expectConfirmationDetail(page, 'Saved Connection / Endpoint', 'Control Gateway')
  await expectConfirmationDetail(page, 'Variable Node', 'Filler Temperature')
  await expectConfirmationDetail(page, 'NodeID', 'ns=2;s=Plant.Line1.Filler.Temperature')
  await expectConfirmationDetail(page, 'Current Live Value', '83.7')
  await expectConfirmationDetail(page, 'Current Status', 'Good')
  await expectConfirmationDetail(page, 'Target Value', '84.2')
  await expectConfirmationDetail(page, 'Data Type', 'Double')
  await expect(page.getByRole('dialog').locator('dt', { hasText: /^(Source|Server) Timestamp$/ })).toHaveCount(2)
  await expect(page.getByRole('dialog').getByText('2026', { exact: false })).toHaveCount(2)
})

test('a changed Live Value invalidates confirmation until it is cancelled and reviewed', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-confirmation-live-value-change')
  await page.getByLabel('Target Value').fill('84.2')
  await writeValue.click()

  const invalidatedWarning = page.getByText('Current Live Value changed while confirmation was open. Cancel and review the new value before writing.')
  await expect(invalidatedWarning).toBeVisible()
  await expect(page.getByRole('button', { name: 'Confirm write' })).toBeDisabled()

  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'This changes the OPC UA Server' })).toBeHidden()
  await expect(page.getByText('84.0', { exact: true })).toBeVisible()
  await expect(writeValue).toBeEnabled()

  await writeValue.click()
  await expectConfirmationDetail(page, 'Current Live Value', '84.0')
  await expect(page.getByRole('button', { name: 'Confirm write' })).toBeEnabled()
})

test('successful Variable Node Write feedback is visible inline and as a toast', async ({ page }) => {
  await stubVariableNodeWrite(page, {
    nodeID: 'ns=2;s=Plant.Line1.Filler.Temperature',
    targetValue: '84.2',
    status: 'success',
    warning: '',
    readBack: { Value: '84.2', Status: 'Good' }
  })

  await submitVariableNodeWrite(page)

  await expect(page.getByText('Write accepted. Read-back: 84.2 (Good)', { exact: true })).toBeVisible()
  await expect(page.getByText('Variable Node Write accepted', { exact: true })).toBeVisible()
})

test('failed Variable Node Write feedback is visible inline and as a toast', async ({ page }) => {
  await stubVariableNodeWrite(page, null, 'BadNotWritable')

  await submitVariableNodeWrite(page)

  await expect(page.getByText('Error: BadNotWritable', { exact: true })).toBeVisible()
  await expect(page.getByText('Variable Node Write failed: Error: BadNotWritable', { exact: true })).toBeVisible()
})

test('read-back mismatch is shown as a warning instead of ordinary success', async ({ page }) => {
  const warning = 'read-back mismatch: target "84.2" but server returned "83.7"'
  await stubVariableNodeWrite(page, {
    nodeID: 'ns=2;s=Plant.Line1.Filler.Temperature',
    targetValue: '84.2',
    status: 'warning',
    warning,
    readBack: { Value: '83.7', Status: 'Good' }
  })

  await submitVariableNodeWrite(page)

  await expect(page.getByText(warning, { exact: true })).toBeVisible()
  await expect(page.getByText(`Variable Node Write accepted with warning: ${warning}`, { exact: true })).toBeVisible()
  await expect(page.getByText(/Write accepted\. Read-back:/)).toHaveCount(0)
})

test('range metadata warning is visible and does not block confirmation', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-confirmation')
  const warning = 'Target Value is above EURange (0–100); this warns but does not block.'

  await page.getByLabel('Target Value').fill('125')
  await expect(page.getByText(warning, { exact: true })).toBeVisible()
  await expect(writeValue).toBeEnabled()

  await writeValue.click()
  await expect(page.getByRole('dialog').getByText(warning, { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Confirm write' })).toBeEnabled()
})

test('non-Good current status warning is visible and does not block confirmation for a fresh Live Value', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-status-warning')
  const warning = 'Current status is UncertainLastUsableValue; confirm the value is safe to change.'

  await page.getByLabel('Target Value').fill('84.2')
  await expect(page.getByText(warning, { exact: true })).toBeVisible()
  await expect(writeValue).toBeEnabled()

  await writeValue.click()
  await expect(page.getByRole('dialog').getByText(warning, { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Confirm write' })).toBeEnabled()
})

test('write-related Diagnostic Log feedback remains visible after a write', async ({ page }) => {
  const messages = [
    'Variable Node Write attempted for ns=2;s=Plant.Line1.Filler.Temperature target "84.2"',
    'Variable Node Write accepted for ns=2;s=Plant.Line1.Filler.Temperature target "84.2"',
    'Variable Node Write read-back for ns=2;s=Plant.Line1.Filler.Temperature: value="84.2" status=Good'
  ]
  await page.addInitScript((writeMessages) => {
    type EventCallback = (...args: unknown[]) => void
    const listeners = new Map<string, Set<EventCallback>>()
    const runtime = {
      EventsOnMultiple(eventName: string, callback: EventCallback) {
        const eventListeners = listeners.get(eventName) ?? new Set<EventCallback>()
        eventListeners.add(callback)
        listeners.set(eventName, eventListeners)
        return () => eventListeners.delete(callback)
      },
      EventsEmit(eventName: string, ...args: unknown[]) {
        listeners.get(eventName)?.forEach(callback => callback(...args))
      }
    }
    ;(window as typeof window & { runtime: typeof runtime; go: unknown }).runtime = runtime
    ;(window as typeof window & { go: unknown }).go = { main: { App: {
      WriteVariableNodeValue: () => {
        writeMessages.forEach((message, index) => runtime.EventsEmit('diagnostic-log-appended', {
          timestamp: `2026-06-25T14:32:2${index}Z`,
          level: 'info',
          message
        }))
        return Promise.resolve({
          nodeID: 'ns=2;s=Plant.Line1.Filler.Temperature',
          targetValue: '84.2',
          status: 'success',
          warning: '',
          readBack: { Value: '84.2', Status: 'Good' }
        })
      }
    } } }
  }, messages)

  await page.goto('/?screenshot=write-feedback-events')
  await page.getByRole('button', { name: 'Diagnostic Logs' }).click()
  for (const message of messages) await expect(page.getByText(message, { exact: true })).toHaveCount(0)

  await page.getByRole('button', { name: 'Address Space' }).click()
  await page.getByLabel('Target Value').fill('84.2')
  await page.getByRole('button', { name: 'Write value', exact: true }).click()
  await page.getByRole('button', { name: 'Confirm write' }).click()
  await page.getByRole('button', { name: 'Diagnostic Logs' }).click()

  for (const message of messages) await expect(page.getByText(message, { exact: true })).toBeVisible()
})

test('Read-Only Mode explains why Variable Node Write is unavailable', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-read-only')

  await expect(writeValue).toBeDisabled()
  await expect(page.getByText('Read-Only Mode is active.')).toBeVisible()
})

test('loading metadata explains that write availability cannot yet be determined and blocks confirmation', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-metadata-loading')

  await page.getByLabel('Target Value').fill('84.2')
  await expect(writeValue).toBeDisabled()
  await expect(page.getByText('Write availability cannot yet be determined while Variable Node metadata is loading.')).toBeVisible()

  await writeValue.evaluate(button => (button as HTMLButtonElement).click())
  await expect(page.getByRole('heading', { name: 'This changes the OPC UA Server' })).toBeHidden()
  await expect(page.getByRole('button', { name: 'Confirm write' })).toHaveCount(0)
})

test('failed metadata explains that write availability could not be determined and blocks confirmation', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-metadata-failed')

  await page.getByLabel('Target Value').fill('84.2')
  await expect(writeValue).toBeDisabled()
  await expect(page.getByText('Write availability could not be determined because Variable Node metadata failed to load.')).toBeVisible()

  await writeValue.evaluate(button => (button as HTMLButtonElement).click())
  await expect(page.getByRole('heading', { name: 'This changes the OPC UA Server' })).toBeHidden()
  await expect(page.getByRole('button', { name: 'Confirm write' })).toHaveCount(0)
})

test('missing metadata explains that write availability cannot be determined and blocks confirmation', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-metadata-unavailable')

  await page.getByLabel('Target Value').fill('84.2')
  await expect(writeValue).toBeDisabled()
  await expect(page.getByText('Write availability cannot yet be determined because Variable Node metadata is unavailable.')).toBeVisible()

  await writeValue.evaluate(button => (button as HTMLButtonElement).click())
  await expect(page.getByRole('heading', { name: 'This changes the OPC UA Server' })).toBeHidden()
  await expect(page.getByRole('button', { name: 'Confirm write' })).toHaveCount(0)
})

test('a stale Live Value explains why Variable Node Write is unavailable', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-live-value-stale')

  await expect(writeValue).toBeDisabled()
  await expect(page.getByText('Current Live Value is stale.')).toBeVisible()
})

test('an unavailable Live Value explains why Variable Node Write is unavailable and blocks confirmation', async ({ page }) => {
  const writeValue = await openVariableNodeInspection(page, 'write-live-value-unavailable')

  await page.getByLabel('Target Value').fill('84.2')
  await expect(writeValue).toBeDisabled()
  await expect(page.getByText('Current Live Value is unavailable.')).toBeVisible()

  await writeValue.evaluate(button => (button as HTMLButtonElement).click())
  await expect(page.getByRole('heading', { name: 'This changes the OPC UA Server' })).toBeHidden()
  await expect(page.getByRole('button', { name: 'Confirm write' })).toHaveCount(0)
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
