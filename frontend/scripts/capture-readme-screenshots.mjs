import { chromium } from 'playwright'
import { mkdir } from 'node:fs/promises'
import { spawn } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const __dirname = dirname(fileURLToPath(import.meta.url))
const frontendDir = resolve(__dirname, '..')
const repoRoot = resolve(frontendDir, '..')
const outputDir = resolve(repoRoot, 'docs/assets/readme')
const port = 4173
const baseURL = `http://127.0.0.1:${port}`

const shots = [
  { name: 'hero.png', mode: 'hero', viewport: { width: 1600, height: 1000 } },
  { name: 'address-space-search.png', mode: 'search', viewport: { width: 1600, height: 1000 } },
  { name: 'variable-node-inspection.png', mode: 'inspection', viewport: { width: 1600, height: 1000 } },
  { name: 'watchlist.png', mode: 'watchlist', viewport: { width: 1600, height: 1000 } },
  { name: 'session-trend.png', mode: 'trend', viewport: { width: 1600, height: 1000 } },
  { name: 'connection-manager.png', mode: 'connections', viewport: { width: 1600, height: 1000 } }
]

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

await mkdir(outputDir, { recursive: true })

const viteBin = resolve(frontendDir, 'node_modules/vite/bin/vite.js')
const server = spawn(process.execPath, [viteBin, '--host', '127.0.0.1', '--port', String(port), '--strictPort'], {
  cwd: frontendDir,
  stdio: ['ignore', 'pipe', 'pipe']
})

let serverExited = false
server.on('exit', code => {
  serverExited = true
  if (code && code !== 0) console.error(`Vite exited with code ${code}`)
})
server.stdout.on('data', chunk => process.stdout.write(chunk))
server.stderr.on('data', chunk => process.stderr.write(chunk))

let browser
try {
  await Promise.race([
    waitForServer(baseURL),
    new Promise((_, reject) => server.once('exit', code => reject(new Error(`Vite exited before serving screenshots with code ${code}`))))
  ])
  if (serverExited) throw new Error('Vite exited before serving screenshots')
  browser = await chromium.launch()
  const page = await browser.newPage({ deviceScaleFactor: 1 })

  for (const shot of shots) {
    await page.setViewportSize(shot.viewport)
    await page.goto(`${baseURL}/?screenshot=${shot.mode}`, { waitUntil: 'networkidle' })
    await page.screenshot({ path: resolve(outputDir, shot.name), fullPage: false })
    console.log(`wrote docs/assets/readme/${shot.name}`)
  }
} finally {
  if (browser) await browser.close()
  server.kill()
}
