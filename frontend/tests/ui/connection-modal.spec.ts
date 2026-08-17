import { expect, test } from 'playwright/test'

test('Connection Manager is a modal and submits the selected secure endpoint configuration', async ({ page }) => {
  await page.addInitScript(() => {
    const testWindow = window as typeof window & { go: unknown; connectRequests: unknown[] }
    testWindow.connectRequests = []
    testWindow.go = { main: { App: {
      Connect: (request: unknown) => {
        testWindow.connectRequests.push(request)
        return Promise.resolve()
      },
      GetSessionSafety: () => Promise.resolve({ connected: true, readOnlyMode: true }),
      GetSavedConnections: () => Promise.resolve([])
    } } }
  })

  await page.goto('/?screenshot=connections')

  const dialog = page.getByRole('dialog', { name: 'Connect to an OPC UA Server' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByLabel('OPC UA Server URL')).toHaveValue('opc.tcp://localhost:4840')
  await expect(dialog.getByRole('radio')).toHaveCount(2)

  await dialog.getByLabel('Client Certificate Path').fill('C:\\certs\\client.crt')
  await dialog.getByLabel('Private Key Path').fill('C:\\certs\\client.key')
  await expect(dialog.getByRole('button', { name: 'Initialize Connection' })).toBeEnabled()
  await dialog.getByRole('button', { name: 'Initialize Connection' }).click()

  await expect(dialog).toHaveCount(0)
  await expect.poll(() => page.evaluate(() => (window as typeof window & { connectRequests: unknown[] }).connectRequests)).toEqual([{
    existingName: '',
    savedConnectionID: '',
    name: '',
    endpoint: 'opc.tcp://localhost:4840',
    securityPolicy: 'Basic256Sha256',
    securityMode: 'SignAndEncrypt',
    authType: 'Anonymous',
    username: '',
    password: '',
    clientCertificatePath: 'C:\\certs\\client.crt',
    clientPrivateKeyPath: 'C:\\certs\\client.key',
    serverThumbprint: '8A:91:4F:2C:67:12:EB:44'
  }])
})

test('Connection Manager restores focus to its opener when closed', async ({ page }) => {
  await page.goto('/?screenshot=hero')

  const managerButton = page.getByRole('button', { name: /Connection Manager/ })
  await managerButton.click()
  const dialog = page.getByRole('dialog', { name: 'Connect to an OPC UA Server' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByLabel('OPC UA Server URL')).toBeFocused()

  await dialog.getByLabel('OPC UA Server URL').press('Escape')
  await expect(dialog).toHaveCount(0)
  await expect(managerButton).toBeFocused()
})
