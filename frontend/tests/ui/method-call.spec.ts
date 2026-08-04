import { expect, test, type Page } from 'playwright/test'

const methodDetails = {
  ObjectNodeID: 'ns=3;s=Demo.CTT.Methods',
  MethodNodeID: 'ns=3;s=Demo.CTT.Methods.MethodIO',
  Description: 'Adds 2 unsigned integers',
  Executable: true,
  UserExecutable: true,
  InputArguments: [
    { Name: 'Summand1', DataType: 'UInt32', DataTypeID: 'i=7', ValueRank: 'Scalar', Description: 'First unsigned integer', ArrayDimensions: [], Supported: true },
    { Name: 'Summand2', DataType: 'UInt32', DataTypeID: 'i=7', ValueRank: 'Scalar', Description: 'Second unsigned integer', ArrayDimensions: [], Supported: true }
  ],
  OutputArguments: [
    { Name: 'Sum', DataType: 'UInt32', DataTypeID: 'i=7', ValueRank: 'Scalar', Description: 'The sum', ArrayDimensions: [], Supported: true }
  ]
}

async function stubMethodAPI(page: Page, options: { details?: unknown; detailsError?: string; result?: unknown; callError?: string; deferredCall?: boolean } = {}) {
  await page.addInitScript(({ details, detailsError, result, callError, deferredCall }) => {
    const testWindow = window as typeof window & { go: unknown; methodCalls: unknown[]; methodDetailRequests: unknown[]; releaseMethodCall?: () => void }
    testWindow.methodCalls = []
    testWindow.methodDetailRequests = []
    const getDetails = (request: unknown) => {
      testWindow.methodDetailRequests.push(request)
      return detailsError ? Promise.reject(new Error(detailsError)) : Promise.resolve(details)
    }
    const call = (request: unknown) => {
      testWindow.methodCalls.push(request)
      if (callError) return Promise.reject(new Error(callError))
      if (deferredCall) return new Promise(resolve => { testWindow.releaseMethodCall = () => resolve(result) })
      return Promise.resolve(result)
    }
    testWindow.go = { main: { App: {
      GetMethodDetails: getDetails,
      CallMethod: call,
      ClearVariableNodeInspection: () => Promise.resolve(),
      Disconnect: () => Promise.resolve(),
      InspectVariableNode: () => Promise.resolve(),
      GetSessionSafety: () => Promise.resolve({ connected: false, readOnlyMode: true })
    } } }
  }, {
    details: options.details ?? methodDetails,
    detailsError: options.detailsError ?? '',
    result: options.result ?? { StatusCode: 'StatusGood', InputArgumentResults: ['StatusGood', 'StatusGood'], OutputArguments: [{ DataType: 'UInt32', Value: 'uint32(42)' }] },
    callError: options.callError ?? '',
    deferredCall: options.deferredCall ?? false
  })
}

async function openMethod(page: Page, source: 'tree' | 'search' = 'tree', state = 'method-call') {
  await page.goto(`/?screenshot=${state}`)
  if (source === 'tree') await page.getByRole('button', { name: 'MethodIO Method', exact: true }).click()
  else {
    await page.getByRole('button', { name: 'Search', exact: true }).click()
    await page.getByRole('button', { name: /^play_circle MethodIO/ }).click()
  }
  await expect(page.getByRole('heading', { name: 'MethodIO', level: 2 })).toBeVisible()
}

async function enterMethodInputs(page: Page) {
  await page.getByLabel('Summand1').fill('20')
  await page.getByLabel('Summand2').fill('22')
}

test('Method Nodes have a distinct affordance and inspection remains available in Read-Only Mode', async ({ page }) => {
  await stubMethodAPI(page)
  await openMethod(page, 'tree', 'method-call-read-only')

  await expect(page.getByText('Method Call', { exact: true })).toBeVisible()
  await expect(page.getByText('Adds 2 unsigned integers', { exact: true })).toBeVisible()
  await expect(page.getByRole('definition').filter({ hasText: methodDetails.ObjectNodeID }).first()).toBeVisible()
  await expect(page.getByRole('definition').filter({ hasText: methodDetails.MethodNodeID }).first()).toBeVisible()
  await expect(page.getByText('Executable for this session', { exact: true })).toBeVisible()
  await expect(page.getByText('First unsigned integer', { exact: true })).toBeVisible()
  await expect(page.getByText('Sum', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Call Method' })).toBeDisabled()
  await expect(page.getByText('Read-Only Mode is active.', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Allow changes this session' })).toBeVisible()
})

test('search activation preserves the owning Object Node and opens the same Method Call panel', async ({ page }) => {
  await stubMethodAPI(page)
  await openMethod(page, 'search')

  await expect.poll(() => page.evaluate(() => (window as typeof window & { methodDetailRequests: unknown[] }).methodDetailRequests)).toEqual([{
    objectNodeID: methodDetails.ObjectNodeID,
    methodNodeID: methodDetails.MethodNodeID
  }])
  await expect(page.getByRole('definition').filter({ hasText: methodDetails.ObjectNodeID }).first()).toBeVisible()
})

test('invalid and unsupported arguments show visible disabled reasons', async ({ page }) => {
  await stubMethodAPI(page, { details: {
    ...methodDetails,
    InputArguments: [
      methodDetails.InputArguments[0],
      { Name: 'Schedule', DataType: 'DateTime', DataTypeID: 'i=13', ValueRank: 'OneDimension', Description: 'Unsupported schedule', ArrayDimensions: [4], Supported: false }
    ]
  } })
  await openMethod(page)

  await expect(page.getByText('Enter a value for Summand1.', { exact: true })).toBeVisible()
  await page.getByLabel('Summand1').fill('4294967296')
  await expect(page.getByText('Summand1 is outside UInt32 range.', { exact: true })).toBeVisible()
  await expect(page.getByLabel('Schedule')).toBeDisabled()
  await expect(page.getByText('Schedule uses unsupported DataType DateTime and ValueRank OneDimension.', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Call Method' })).toBeDisabled()
})

test('Float and Double arguments accept valid forms and reject overflow and underflow', async ({ page }) => {
  await stubMethodAPI(page, { details: {
    ...methodDetails,
    InputArguments: [
      { ...methodDetails.InputArguments[0], Name: 'FloatValue', DataType: 'Float', DataTypeID: 'i=10' },
      { ...methodDetails.InputArguments[1], Name: 'DoubleValue', DataType: 'Double', DataTypeID: 'i=11' }
    ]
  } })
  await openMethod(page)

  const callMethod = page.getByRole('button', { name: 'Call Method' })
  await page.getByLabel('FloatValue').fill('1.25e-3')
  await page.getByLabel('DoubleValue').fill('-.5E+2')
  await expect(callMethod).toBeEnabled()

  await page.getByLabel('FloatValue').fill('1e50')
  await expect(page.getByText('FloatValue must parse as Float.', { exact: true })).toBeVisible()
  await expect(callMethod).toBeDisabled()
  await page.getByLabel('FloatValue').fill('1e-50')
  await expect(page.getByText('FloatValue must parse as Float.', { exact: true })).toBeVisible()
  await expect(callMethod).toBeDisabled()

  await page.getByLabel('FloatValue').fill('3.5')
  await page.getByLabel('DoubleValue').fill('1e999')
  await expect(page.getByText('DoubleValue must parse as Double.', { exact: true })).toBeVisible()
  await expect(callMethod).toBeDisabled()
  await page.getByLabel('DoubleValue').fill('1e-999')
  await expect(page.getByText('DoubleValue must parse as Double.', { exact: true })).toBeVisible()
  await expect(callMethod).toBeDisabled()
})

test('loading, failed metadata, and non-executable states explain why calling is blocked', async ({ page }) => {
  await page.addInitScript(() => {
    const testWindow = window as typeof window & { go: unknown }
    testWindow.go = { main: { App: {
      GetMethodDetails: () => new Promise(() => {}),
      ClearVariableNodeInspection: () => Promise.resolve()
    } } }
  })
  await page.goto('/?screenshot=method-call')
  await page.getByRole('button', { name: 'MethodIO Method', exact: true }).click()
  await expect(page.getByText('Method metadata is loading.', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Call Method' })).toBeDisabled()

  await stubMethodAPI(page, { detailsError: 'BadSecurityModeInsufficient' })
  await openMethod(page)
  await expect(page.getByText('Method metadata failed to load: Error: BadSecurityModeInsufficient', { exact: true })).toBeVisible()

  await stubMethodAPI(page, { details: { ...methodDetails, UserExecutable: false } })
  await openMethod(page)
  await expect(page.getByText('Method is not executable for the current session.', { exact: true })).toBeVisible()

  await stubMethodAPI(page, { details: { ...methodDetails, Executable: false } })
  await openMethod(page)
  await expect(page.getByText('Method is not executable.', { exact: true })).toBeVisible()
})

test('Enter never submits and an in-flight Method call disables repeated submission', async ({ page }) => {
  await stubMethodAPI(page, { deferredCall: true })
  await openMethod(page)
  await enterMethodInputs(page)
  await page.getByLabel('Summand2').press('Enter')
  await expect.poll(() => page.evaluate(() => (window as typeof window & { methodCalls: unknown[] }).methodCalls.length)).toBe(0)

  await page.getByRole('button', { name: 'Call Method' }).click()
  await expect(page.getByRole('button', { name: 'Calling…' })).toBeDisabled()
  await expect(page.getByText('Method call is in progress.', { exact: true })).toBeVisible()
  await expect.poll(() => page.evaluate(() => (window as typeof window & { methodCalls: unknown[] }).methodCalls.length)).toBe(1)
  await page.evaluate(() => (window as typeof window & { releaseMethodCall?: () => void }).releaseMethodCall?.())
  await expect(page.getByText('StatusGood', { exact: true }).first()).toBeVisible()
})

test('a Good result displays ordered raw outputs and input changes clear stale results', async ({ page }) => {
  await stubMethodAPI(page)
  await openMethod(page)
  await enterMethodInputs(page)

  const review = page.getByRole('region', { name: 'Method call review' })
  await expect(review.getByText('Control Gateway', { exact: true })).toBeVisible()
  await expect(review.getByText('20', { exact: true })).toBeVisible()
  await expect(review.getByText('22', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Call Method' }).click()

  await expect.poll(() => page.evaluate(() => (window as typeof window & { methodCalls: unknown[] }).methodCalls)).toEqual([{
    objectNodeID: methodDetails.ObjectNodeID,
    methodNodeID: methodDetails.MethodNodeID,
    inputArguments: ['20', '22']
  }])
  await expect(page.getByText('StatusGood', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('uint32(42)', { exact: true })).toBeVisible()
  await expect(page.getByText('Method call completed: StatusGood', { exact: true })).toBeVisible()
  await page.getByLabel('Summand1').fill('21')
  await expect(page.getByText('uint32(42)', { exact: true })).toHaveCount(0)
})

test('selecting another Address Space node clears the Method result and transient inputs', async ({ page }) => {
  await stubMethodAPI(page)
  await openMethod(page)
  await enterMethodInputs(page)
  await page.getByRole('button', { name: 'Call Method' }).click()
  await expect(page.getByText('uint32(42)', { exact: true })).toBeVisible()

  await page.getByRole('button', { name: 'Filler Temperature Variable', exact: true }).click()

  await expect(page.getByRole('heading', { name: 'MethodIO', level: 2 })).toHaveCount(0)
  await expect(page.getByText('uint32(42)', { exact: true })).toHaveCount(0)
  await expect(page.getByLabel('Summand1')).toHaveCount(0)
})

test('non-Good and transport failures remain visible inline with concise toasts', async ({ page }) => {
  await stubMethodAPI(page, { result: { StatusCode: 'BadInvalidArgument (0x80AB0000)', InputArgumentResults: ['BadTypeMismatch'], OutputArguments: [] } })
  await openMethod(page)
  await enterMethodInputs(page)
  await page.getByRole('button', { name: 'Call Method' }).click()
  await expect(page.getByText('BadInvalidArgument (0x80AB0000)', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('Method call returned BadInvalidArgument (0x80AB0000)', { exact: true })).toBeVisible()

  await stubMethodAPI(page, { callError: 'transport unavailable' })
  await openMethod(page)
  await enterMethodInputs(page)
  await page.getByRole('button', { name: 'Call Method' }).click()
  await expect(page.getByText('Method call failed: Error: transport unavailable', { exact: true }).first()).toBeVisible()
})

test('a lost session keeps the Method visible but blocks execution at the Wails seam', async ({ page }) => {
  await page.addInitScript(({ details }) => {
    type EventCallback = (...args: unknown[]) => void
    const listeners = new Map<string, Set<EventCallback>>()
    const testWindow = window as typeof window & { go: unknown; runtime: unknown; methodCalls: unknown[]; loseSession: () => void }
    testWindow.methodCalls = []
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
    testWindow.runtime = runtime
    testWindow.loseSession = () => runtime.EventsEmit('session-safety-updated', { connected: false, readOnlyMode: false })
    testWindow.go = { main: { App: {
      GetDiagnosticLogs: () => Promise.resolve([]),
      GetSavedConnections: () => Promise.resolve([]),
      GetSessionSafety: () => Promise.resolve({ connected: true, readOnlyMode: false }),
      GetSessionTrend: () => Promise.resolve({ nodes: [], points: [] }),
      GetWatchlist: () => Promise.resolve([]),
      GetMethodDetails: () => Promise.resolve(details),
      CallMethod: (request: unknown) => { testWindow.methodCalls.push(request); return Promise.resolve({ StatusCode: 'StatusGood', InputArgumentResults: [], OutputArguments: [] }) },
      ClearVariableNodeInspection: () => Promise.resolve()
    } } }
  }, { details: methodDetails })
  await openMethod(page, 'tree', 'method-call-events')
  await enterMethodInputs(page)
  await page.evaluate(() => (window as typeof window & { loseSession: () => void }).loseSession())

  await expect(page.getByRole('heading', { name: 'MethodIO', level: 2 })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Call Method' })).toBeDisabled()
  await expect(page.getByText('Connect to an OPC UA Server before calling a Method.', { exact: true })).toBeVisible()
  await expect(page.getByText('Read-Only Mode is active.', { exact: true })).toHaveCount(0)
  await expect.poll(() => page.evaluate(() => (window as typeof window & { methodCalls: unknown[] }).methodCalls)).toEqual([])
})

test('disconnect clears the Method Call panel and transient inputs', async ({ page }) => {
  await stubMethodAPI(page)
  await openMethod(page)
  await enterMethodInputs(page)
  await page.getByRole('button', { name: 'Call Method' }).click()
  await expect(page.getByText('uint32(42)', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Disconnect' }).click()

  await expect(page.getByRole('heading', { name: 'MethodIO', level: 2 })).toHaveCount(0)
  await expect(page.getByLabel('Summand1')).toHaveCount(0)
  await expect(page.getByText('uint32(42)', { exact: true })).toHaveCount(0)
  await expect(page.getByRole('heading', { name: 'Connect to an OPC UA Server' })).toBeVisible()
})
