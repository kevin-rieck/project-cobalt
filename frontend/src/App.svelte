<script lang="ts">
  import { onMount } from 'svelte'
  import { BrowseChildren, ClearVariableNodeInspection, Connect, Disconnect, DiscoverEndpoints, GetDiagnosticLogs, InspectVariableNode } from '../wailsjs/go/main/App.js'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'

  type Tab = 'connections' | 'address-space' | 'live-monitor' | 'trends' | 'logs'
  type AuthType = 'Anonymous' | 'UserName'

  type Endpoint = {
    URL: string
    SecurityPolicy: string
    SecurityMode: string
    SecurityLevel: number
    UserTokenTypes: string[]
    ServerThumbprint: string
  }

  type AddressNode = {
    NodeID: string
    DisplayName: string
    BrowseName: string
    NodeClass: string
  }

  type TreeNode = {
    node: AddressNode
    depth: number
    expanded: boolean
    childrenLoaded: boolean
    loading: boolean
    error: string
  }

  type Inspection = {
    node: AddressNode
    value: { Value: string; Status: string; SourceTimestamp: string; ServerTimestamp: string }
    details: {
      NodeID: string
      Description: string
      DataType: string
      AccessLevel: string
      Writable: boolean
      ValueRank: string
      ArrayDimensions: string
      EngineeringUnit: string
      EURange?: { Low: number; High: number }
      InstrumentRange?: { Low: number; High: number }
    }
    subscribing: boolean
    loadingDetails: boolean
    stale: boolean
    outOfRange: string
    updateCount: number
    error: string
    detailsError: string
  }

  type DiagnosticLogEntry = { timestamp: string; level: string; message: string }

  const objectsRoot: TreeNode = {
    node: { NodeID: 'i=85', DisplayName: 'Objects', BrowseName: 'Objects', NodeClass: 'Object' },
    depth: 0,
    expanded: false,
    childrenLoaded: false,
    loading: false,
    error: ''
  }

  let activeTab: Tab = 'connections'
  let endpointText = 'opc.tcp://localhost:4840'
  let endpoints: Endpoint[] = []
  let selectedEndpoint = 0
  let authType: AuthType = 'Anonymous'
  let username = ''
  let password = ''
  let discovering = false
  let connecting = false
  let connected = false
  let connectionError = ''
  let currentConnection = ''
  let tree: TreeNode[] = [{ ...objectsRoot }]
  let selectedNodeID = ''
  let inspection: Inspection | null = null
  let logs: DiagnosticLogEntry[] = []
  let toasts: { id: number; level: string; message: string }[] = []

  $: selectedEndpointInfo = endpoints[selectedEndpoint]
  $: canUseUsername = selectedEndpointInfo?.UserTokenTypes?.some(token => token.includes('UserName')) ?? false
  $: if (!canUseUsername && authType === 'UserName') authType = 'Anonymous'
  $: visibleTree = tree.filter((_, index) => !isHidden(index))

  onMount(async () => {
    logs = await GetDiagnosticLogs()
    const offInspection = EventsOn('variable-inspection-updated', (payload: Inspection | null) => {
      inspection = payload
    })
    const offLog = EventsOn('diagnostic-log-appended', (entry: DiagnosticLogEntry) => {
      logs = [...logs, entry].slice(-500)
    })
    return () => {
      offInspection()
      offLog()
    }
  })

  function addToast(level: string, message: string) {
    const id = Date.now() + Math.random()
    toasts = [...toasts, { id, level, message }]
    setTimeout(() => {
      toasts = toasts.filter(toast => toast.id !== id)
    }, 4500)
  }

  async function discover() {
    discovering = true
    connectionError = ''
    endpoints = []
    selectedEndpoint = 0
    try {
      endpoints = await DiscoverEndpoints(endpointText)
      if (endpoints.length === 0) {
        connectionError = 'No endpoints were advertised by this OPC UA Server.'
        addToast('error', connectionError)
      }
    } catch (error) {
      connectionError = String(error)
      addToast('error', connectionError)
    } finally {
      discovering = false
    }
  }

  async function connect() {
    if (!selectedEndpointInfo) return
    connecting = true
    connectionError = ''
    try {
      await Connect({
        endpoint: endpointText,
        securityPolicy: selectedEndpointInfo.SecurityPolicy,
        securityMode: selectedEndpointInfo.SecurityMode,
        authType,
        username,
        password
      })
      connected = true
      currentConnection = endpointText
      tree = [{ ...objectsRoot }]
      selectedNodeID = ''
      inspection = null
      activeTab = 'address-space'
      addToast('info', 'Connected')
    } catch (error) {
      connectionError = String(error)
      addToast('error', connectionError)
    } finally {
      connecting = false
    }
  }

  async function disconnect() {
    try {
      await Disconnect()
      connected = false
      currentConnection = ''
      tree = [{ ...objectsRoot }]
      selectedNodeID = ''
      inspection = null
      activeTab = 'connections'
      addToast('info', 'Disconnected')
    } catch (error) {
      addToast('error', String(error))
    }
  }

  async function toggleNode(item: TreeNode) {
    const index = tree.findIndex(entry => entry.node.NodeID === item.node.NodeID)
    if (index < 0) return
    if (tree[index].childrenLoaded) {
      tree[index].expanded = !tree[index].expanded
      tree = [...tree]
      return
    }
    tree[index].loading = true
    tree[index].error = ''
    tree = [...tree]
    try {
      const children: AddressNode[] = await BrowseChildren(item.node.NodeID)
      const childNodes: TreeNode[] = children.map(child => ({ node: child, depth: item.depth + 1, expanded: false, childrenLoaded: false, loading: false, error: '' }))
      const end = subtreeEnd(index)
      tree = [...tree.slice(0, index + 1), ...childNodes, ...tree.slice(end)]
      tree[index].expanded = true
      tree[index].childrenLoaded = true
      tree[index].loading = false
      tree = [...tree]
    } catch (error) {
      tree[index].loading = false
      tree[index].error = String(error)
      tree = [...tree]
      addToast('error', `Browse failed: ${String(error)}`)
    }
  }

  async function selectNode(item: TreeNode) {
    selectedNodeID = item.node.NodeID
    if (item.node.NodeClass === 'Variable') {
      await InspectVariableNode(item.node)
    } else {
      await ClearVariableNodeInspection()
    }
  }

  function isHidden(index: number) {
    const item = tree[index]
    for (let cursor = index - 1; cursor >= 0; cursor--) {
      const ancestor = tree[cursor]
      if (ancestor.depth < item.depth) {
        if (!ancestor.expanded) return true
      }
    }
    return false
  }

  function subtreeEnd(index: number) {
    const depth = tree[index].depth
    let end = index + 1
    while (end < tree.length && tree[end].depth > depth) end++
    return end
  }

  function compactTime(value: string) {
    if (!value || value.startsWith('0001-')) return '—'
    return new Date(value).toLocaleTimeString()
  }

  function navClass(tab: Tab) {
    return activeTab === tab
      ? 'bg-secondary-container text-on-secondary-container border-primary'
      : 'text-on-surface-variant border-transparent hover:bg-surface-container-highest'
  }
</script>

<div class="flex h-screen overflow-hidden bg-background text-on-background">
  <aside class="flex h-screen w-sidebar-width shrink-0 flex-col border-r border-outline-variant bg-surface-container py-md">
    <div class="border-b border-outline-variant px-lg pb-lg">
      <div class="flex items-center gap-sm">
        <div class="flex h-8 w-8 items-center justify-center rounded border border-outline-variant bg-surface-container-highest">
          <span class="material-symbols-outlined text-primary">dns</span>
        </div>
        <div class="min-w-0">
          <h1 class="truncate text-xl font-black tracking-tight text-secondary">OPC UA Studio</h1>
          <p class="flex items-center gap-xs truncate text-sm text-on-surface-variant"><span class="h-2 w-2 rounded-full {connected ? 'bg-emerald-400' : 'bg-outline'}"></span>{connected ? currentConnection : 'Disconnected'}</p>
        </div>
      </div>
    </div>

    <nav class="flex-1 space-y-xs overflow-y-auto px-md py-md">
      <button class="flex w-full items-center gap-md rounded border-l-2 px-md py-sm text-left transition-colors {navClass('connections')}" on:click={() => (activeTab = 'connections')}>
        <span class="material-symbols-outlined">settings_input_component</span><span class="label text-current">Connection Manager</span>
      </button>
      <button class="flex w-full items-center gap-md rounded border-l-2 px-md py-sm text-left transition-colors {navClass('address-space')}" on:click={() => (activeTab = 'address-space')}>
        <span class="material-symbols-outlined">account_tree</span><span class="label text-current">Address Space</span>
      </button>
      <button class="flex w-full items-center gap-md rounded border-l-2 px-md py-sm text-left transition-colors {navClass('live-monitor')}" on:click={() => (activeTab = 'live-monitor')}>
        <span class="material-symbols-outlined">analytics</span><span class="label text-current">Live Monitor</span>
      </button>
      <button class="flex w-full items-center gap-md rounded border-l-2 px-md py-sm text-left transition-colors {navClass('trends')}" on:click={() => (activeTab = 'trends')}>
        <span class="material-symbols-outlined">show_chart</span><span class="label text-current">Trend Dashboard</span>
      </button>
    </nav>

    <div class="space-y-sm border-t border-outline-variant px-md pt-md">
      <button class="btn-secondary flex w-full items-center justify-center gap-sm" disabled><span class="material-symbols-outlined text-sm">add</span>Add New Server</button>
      <button class="flex w-full items-center gap-md rounded px-md py-sm text-left text-on-surface-variant transition-colors hover:bg-surface-container-highest" on:click={() => (activeTab = 'logs')}>
        <span class="material-symbols-outlined">terminal</span><span class="label text-current">Diagnostic Logs</span>
      </button>
    </div>
  </aside>

  <div class="flex min-w-0 flex-1 flex-col">
    <header class="flex h-12 shrink-0 items-center justify-between border-b border-outline-variant bg-surface px-md">
      <div class="flex items-center gap-lg">
        <span class="text-xl font-bold tracking-tight text-primary">OPC UA Studio</span>
        <div class="hidden w-72 items-center rounded border border-outline-variant bg-surface-container px-sm py-xs md:flex">
          <span class="material-symbols-outlined mr-sm text-[18px] text-on-surface-variant">search</span>
          <input class="w-full bg-transparent text-sm outline-none placeholder:text-on-surface-variant" placeholder="Search Address Space..." />
        </div>
      </div>
      <div class="flex items-center gap-sm">
        {#if connected}
          <button class="btn-secondary" on:click={disconnect}>Disconnect</button>
        {:else}
          <button class="btn-primary" on:click={() => (activeTab = 'connections')}>Connect</button>
        {/if}
      </div>
    </header>

    <main class="min-h-0 flex-1 overflow-auto bg-background p-margin-desktop">
      {#if activeTab === 'connections'}
        <section class="mx-auto max-w-5xl space-y-lg">
          <div>
            <p class="label">Connection Manager</p>
            <h2 class="mt-xs text-3xl font-semibold">Connect to an OPC UA Server</h2>
            <p class="mt-sm text-on-surface-variant">Manual endpoint discovery and read-only connection for the first desktop slice.</p>
          </div>

          <div class="panel p-lg">
            <div class="grid gap-md lg:grid-cols-[1fr_auto]">
              <label class="space-y-xs">
                <span class="label">Endpoint</span>
                <input class="field w-full" bind:value={endpointText} placeholder="opc.tcp://host:4840" />
              </label>
              <div class="flex items-end">
                <button class="btn-primary h-9" on:click={discover} disabled={discovering}>{discovering ? 'Discovering…' : 'Discover Endpoints'}</button>
              </div>
            </div>
            {#if connectionError}
              <div class="mt-md rounded border border-error-container bg-error-container/20 p-md text-sm text-error">{connectionError}</div>
            {/if}
          </div>

          {#if endpoints.length > 0}
            <div class="grid gap-lg lg:grid-cols-[1.2fr_0.8fr]">
              <div class="panel overflow-hidden">
                <div class="border-b border-outline-variant p-md"><span class="label">Advertised Endpoints</span></div>
                <div class="max-h-96 overflow-auto">
                  {#each endpoints as endpoint, index}
                    <button class="block w-full border-b border-outline-variant p-md text-left transition-colors hover:bg-surface-container-high {selectedEndpoint === index ? 'bg-secondary-container/60' : ''}" on:click={() => (selectedEndpoint = index)}>
                      <div class="flex items-center justify-between gap-md">
                        <span class="font-mono text-sm text-on-surface">{endpoint.SecurityPolicy} / {endpoint.SecurityMode}</span>
                        <span class="rounded bg-surface-container-highest px-sm py-xs font-mono text-xs text-primary">L{endpoint.SecurityLevel}</span>
                      </div>
                      <p class="mt-xs truncate text-sm text-on-surface-variant">{endpoint.URL}</p>
                      <p class="mt-xs text-xs text-on-surface-variant">Auth: {endpoint.UserTokenTypes.join(', ') || 'Unknown'}</p>
                    </button>
                  {/each}
                </div>
              </div>

              <div class="panel space-y-md p-lg">
                <div>
                  <span class="label">Authentication</span>
                  <div class="mt-sm grid grid-cols-2 gap-sm">
                    <button class="btn-secondary {authType === 'Anonymous' ? 'bg-secondary-container text-on-secondary-container' : ''}" on:click={() => (authType = 'Anonymous')}>Anonymous</button>
                    <button class="btn-secondary {authType === 'UserName' ? 'bg-secondary-container text-on-secondary-container' : ''}" disabled={!canUseUsername} on:click={() => (authType = 'UserName')}>Username</button>
                  </div>
                </div>
                {#if authType === 'UserName'}
                  <label class="block space-y-xs"><span class="label">Username</span><input class="field w-full" bind:value={username} /></label>
                  <label class="block space-y-xs"><span class="label">Password</span><input class="field w-full" type="password" bind:value={password} /></label>
                {/if}
                <button class="btn-primary w-full" on:click={connect} disabled={connecting}>{connecting ? 'Connecting…' : 'Connect'}</button>
                <p class="text-sm text-on-surface-variant">Secure endpoints requiring client certificates are intentionally deferred in this slice.</p>
              </div>
            </div>
          {/if}
        </section>
      {:else if activeTab === 'address-space'}
        <section class="grid h-full min-h-[600px] gap-lg xl:grid-cols-[minmax(360px,0.9fr)_1.1fr]">
          <div class="panel flex min-h-0 flex-col overflow-hidden">
            <div class="flex items-center justify-between border-b border-outline-variant p-md">
              <div><p class="label">Address Space</p><h2 class="text-xl font-semibold">Objects</h2></div>
              {#if !connected}<span class="text-sm text-on-surface-variant">Disconnected</span>{/if}
            </div>
            <div class="min-h-0 flex-1 overflow-auto p-sm">
              {#if !connected}
                <div class="p-lg text-on-surface-variant">Connect to an OPC UA Server to browse its Address Space.</div>
              {:else}
                {#each visibleTree as item}
                  <div class="group flex items-center gap-xs rounded px-sm py-xs hover:bg-surface-container-high" style={`padding-left: ${8 + item.depth * 18}px`}>
                    <button class="flex h-6 w-6 items-center justify-center rounded hover:bg-surface-container-highest" on:click={() => toggleNode(item)} title="Expand or collapse">
                      {#if item.loading}<span class="text-xs text-primary">…</span>{:else}<span class="material-symbols-outlined text-[18px]">{item.expanded ? 'expand_more' : 'chevron_right'}</span>{/if}
                    </button>
                    <button class="min-w-0 flex-1 truncate rounded px-xs py-xs text-left {selectedNodeID === item.node.NodeID ? 'bg-secondary-container text-on-secondary-container' : 'text-on-surface'}" on:click={() => selectNode(item)}>
                      <span class="mr-sm font-medium">{item.node.DisplayName}</span><span class="font-mono text-xs text-on-surface-variant">{item.node.NodeClass}</span>
                    </button>
                  </div>
                  {#if item.error}<div class="ml-lg text-xs text-error">{item.error}</div>{/if}
                {/each}
              {/if}
            </div>
          </div>

          <div class="panel flex min-h-0 flex-col overflow-hidden">
            <div class="border-b border-outline-variant p-md"><p class="label">Variable Node Inspection</p><h2 class="text-xl font-semibold">{inspection?.node?.DisplayName ?? 'No Variable Node selected'}</h2></div>
            <div class="min-h-0 flex-1 overflow-auto p-lg">
              {#if inspection}
                <div class="grid gap-md lg:grid-cols-3">
                  <div class="panel bg-surface-container-low p-md"><p class="label">Live Value</p><p class="mt-sm font-mono text-2xl text-primary">{inspection.value?.Value || '—'}</p></div>
                  <div class="panel bg-surface-container-low p-md"><p class="label">Status</p><p class="mt-sm font-mono text-sm {inspection.stale ? 'text-tertiary' : 'text-emerald-400'}">{inspection.stale ? 'Stale' : inspection.value?.Status || 'Waiting'}</p></div>
                  <div class="panel bg-surface-container-low p-md"><p class="label">Updates</p><p class="mt-sm font-mono text-2xl">{inspection.updateCount}</p></div>
                </div>
                {#if inspection.outOfRange}<div class="mt-md rounded border border-tertiary-container bg-tertiary-container/10 p-md text-tertiary">Out-of-Range: {inspection.outOfRange}</div>{/if}
                {#if inspection.error}<div class="mt-md rounded border border-error-container bg-error-container/20 p-md text-error">{inspection.error}</div>{/if}
                <div class="mt-lg grid gap-md lg:grid-cols-2">
                  <div class="space-y-sm">
                    <p class="label">Metadata</p>
                    <dl class="space-y-xs text-sm">
                      <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">NodeId</dt><dd class="font-mono">{inspection.node.NodeID}</dd></div>
                      <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">BrowseName</dt><dd class="font-mono">{inspection.node.BrowseName}</dd></div>
                      <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Data Type</dt><dd>{inspection.details?.DataType || '—'}</dd></div>
                      <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Access</dt><dd>{inspection.details?.AccessLevel || '—'}</dd></div>
                      <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Engineering Unit</dt><dd>{inspection.details?.EngineeringUnit || '—'}</dd></div>
                    </dl>
                  </div>
                  <div class="space-y-sm">
                    <p class="label">Timestamps</p>
                    <dl class="space-y-xs text-sm">
                      <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Source</dt><dd class="font-mono">{compactTime(inspection.value?.SourceTimestamp)}</dd></div>
                      <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Server</dt><dd class="font-mono">{compactTime(inspection.value?.ServerTimestamp)}</dd></div>
                      <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">EURange</dt><dd class="font-mono">{inspection.details?.EURange ? `${inspection.details.EURange.Low}–${inspection.details.EURange.High}` : '—'}</dd></div>
                    </dl>
                  </div>
                </div>
              {:else}
                <div class="flex h-full items-center justify-center text-on-surface-variant">Select a Variable Node from the Address Space to inspect its Live Value and metadata.</div>
              {/if}
            </div>
          </div>
        </section>
      {:else if activeTab === 'live-monitor' || activeTab === 'trends'}
        <section class="panel flex min-h-[520px] items-center justify-center p-xl text-center">
          <div><span class="material-symbols-outlined text-5xl text-primary">construction</span><h2 class="mt-md text-2xl font-semibold">Coming soon</h2><p class="mt-sm max-w-md text-on-surface-variant">{activeTab === 'live-monitor' ? 'Watchlist-based Live Monitor' : 'Session Trend dashboard'} will build on the Variable Node Inspection slice.</p></div>
        </section>
      {:else if activeTab === 'logs'}
        <section class="panel overflow-hidden">
          <div class="border-b border-outline-variant p-md"><p class="label">Diagnostic Logs</p></div>
          <div class="max-h-[70vh] overflow-auto p-md font-mono text-sm">
            {#each logs as log}
              <div class="grid grid-cols-[170px_70px_1fr] gap-md border-b border-outline-variant/50 py-xs"><span class="text-on-surface-variant">{log.timestamp}</span><span class={log.level === 'error' ? 'text-error' : 'text-primary'}>{log.level}</span><span>{log.message}</span></div>
            {/each}
          </div>
        </section>
      {/if}
    </main>
  </div>

  <div class="pointer-events-none fixed right-md top-md z-50 space-y-sm">
    {#each toasts as toast}
      <div class="w-96 rounded border border-outline-variant bg-surface-container-high p-md text-sm shadow-xl {toast.level === 'error' ? 'text-error' : 'text-on-surface'}">{toast.message}</div>
    {/each}
  </div>
</div>
