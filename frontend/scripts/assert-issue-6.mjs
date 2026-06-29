import { chromium } from 'playwright'
import { spawn } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const __dirname = dirname(fileURLToPath(import.meta.url))
const frontendDir = resolve(__dirname, '..')
const port = 4174
const baseURL = `http://127.0.0.1:${port}`

async function waitForServer(url, timeoutMs = 30_000) {
  const startedAt = Date.now()
  while (Date.now() - startedAt < timeoutMs) {
    try {
      const response = await fetch(url)
      if (response.ok) return
    } catch (_) {
      // Vite is still starting.
    }
    await new Promise(resolve => setTimeout(resolve, 250))
  }
  throw new Error(`Timed out waiting for ${url}`)
}

async function main() {
  const viteBin = resolve(frontendDir, 'node_modules/vite/bin/vite.js')
  const server = spawn(process.execPath, [viteBin, '--host', '127.0.0.1', '--port', String(port), '--strictPort'], {
    cwd: frontendDir,
    stdio: ['ignore', 'pipe', 'pipe']
  })

  let browser
  try {
    await Promise.race([
      waitForServer(baseURL),
      new Promise((_, reject) => server.once('exit', code => reject(new Error(`Vite exited before serving issue #6 checks with code ${code}`))))
    ])

    browser = await chromium.launch()
    const page = await browser.newPage({ viewport: { width: 1600, height: 1000 } })
    await page.goto(`${baseURL}/?screenshot=connections`, { waitUntil: 'networkidle' })

    for (const obsoleteLabel of ['Add New Server', 'New Saved Connection', 'Create Saved Connection', 'Save Connection', 'Save Changes']) {
      if (await page.getByText(obsoleteLabel, { exact: true }).count()) {
        throw new Error(`Connection Manager has obsolete competing create action: ${obsoleteLabel}`)
      }
    }
    const saveCheckbox = page.getByRole('checkbox', { name: /Save as Saved Connection/ })
    if (!(await saveCheckbox.count())) {
      throw new Error('Connection form must expose Save as Saved Connection as the canonical save path')
    }
    if (await saveCheckbox.isChecked()) {
      throw new Error('Manual one-off connect must be the default')
    }
    await page.getByRole('button', { name: /Control Gateway Anonymous/ }).click()
    if (!(await page.getByRole('checkbox', { name: /Update this Saved Connection after successful connect/ }).count())) {
      throw new Error('Selecting a Saved Connection must switch the checkbox to the update path')
    }
    if (!(await page.getByText('Server certificate thumbprint: 8A:91:4F:2C:67:12:EB:44').count())) {
      throw new Error('Saved Connection must show available server certificate thumbprint')
    }
    if (await page.getByText('Server cert:').count()) {
      throw new Error('Server certificate copy must say thumbprint, not Server cert')
    }

    await page.goto(`${baseURL}/?screenshot=connections-empty`, { waitUntil: 'networkidle' })
    if (!(await page.getByText('No Saved Connections yet.').count())) {
      throw new Error('Empty state must say there are no Saved Connections yet')
    }
    if (!(await page.getByText('Use the connection form below and check Save as Saved Connection before connecting.').count())) {
      throw new Error('Empty state must point to the canonical checkbox save path')
    }
  } finally {
    if (browser) await browser.close()
    server.kill()
  }
}

main().catch(error => {
  console.error(error)
  process.exit(1)
})
