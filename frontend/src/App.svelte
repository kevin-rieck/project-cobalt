<script lang="ts">
  import { onMount } from 'svelte'
  import { BrowseChildren, ClearVariableNodeInspection, Connect, DeleteSavedConnection, Disconnect, DiscoverEndpoints, GetDiagnosticLogs, GetSavedConnections, GetSessionTrend, GetWatchlist, InspectVariableNode, PickClientCertificate, PickClientPrivateKey, SaveSavedConnection, SearchAddressSpace, UnwatchVariableNode, WatchVariableNode } from '../wailsjs/go/main/App.js'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'
  import { getReadmeScreenshotState } from './readmeScreenshots'

  type Tab = 'connections' | 'address-space' | 'watchlist' | 'session-trend' | 'logs'
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

  type AddressSpaceSearchResult = {
    node: AddressNode
    matchKind: string
    matchText: string
    source: string
    score: number
  }

  type AddressSpaceSearchView = {
    query: string
    results: AddressSpaceSearchResult[]
    status: string
  }

  type TreeNode = {
    key: string
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
    watched: boolean
    error: string
    detailsError: string
  }

  type WatchlistRow = {
    node: AddressNode
    value: { Value: string; Status: string; SourceTimestamp: string; ServerTimestamp: string }
    dataType: string
    engineeringUnit: string
    stale: boolean
    outOfRange: string
    updateCount: number
    error: string
    detailsError: string
  }

  type SessionTrendPoint = {
    value: string
    status: string
    timestamp: string
    sourceTimestamp: string
    serverTimestamp: string
    receivedAt: string
  }

  type SessionTrendNode = {
    node: AddressNode
    latestValue: string
    status: string
    pointCount: number
  }

  type SessionTrendView = {
    nodes: SessionTrendNode[]
    points: SessionTrendPoint[]
  }

  type DiagnosticLogEntry = { timestamp: string; level: string; message: string }

  type SavedConnection = {
    id: string
    name: string
    endpoint: string
    securityPolicy: string
    securityMode: string
    authType: string
    username?: string
    clientCertificatePath?: string
    clientPrivateKeyPath?: string
    serverCertificateThumbprint?: string
    createdAt: string
    updatedAt: string
    lastConnectedAt?: string
  }

  const readmeScreenshotState = getReadmeScreenshotState()

  const objectsRoot: TreeNode = {
    key: 'root:i=85',
    node: { NodeID: 'i=85', DisplayName: 'Objects', BrowseName: 'Objects', NodeClass: 'Object' },
    depth: 0,
    expanded: false,
    childrenLoaded: false,
    loading: false,
    error: ''
  }

  let activeTab: Tab = readmeScreenshotState?.activeTab ?? 'connections'
  let connectionName = ''
  let endpointText = 'opc.tcp://localhost:4840'
  let endpoints: Endpoint[] = (readmeScreenshotState?.endpoints as Endpoint[]) ?? []
  let selectedEndpoint = 0
  let authType: AuthType = 'Anonymous'
  let username = ''
  let password = ''
  let clientCertificatePath = ''
  let clientPrivateKeyPath = ''
  let discovering = false
  let connecting = false
  let connected = readmeScreenshotState?.connected ?? false
  let connectionError = ''
  let currentConnection = readmeScreenshotState?.currentConnection ?? ''
  let savedConnections: SavedConnection[] = (readmeScreenshotState?.savedConnections as SavedConnection[]) ?? []
  let saveConnectionOnConnect = false
  let savingConnection = false
  let deletingSavedConnectionID = ''
  let editingSavedConnectionID = ''
  let editingSavedConnectionName = ''
  let tree: TreeNode[] = (readmeScreenshotState?.tree as TreeNode[]) ?? [{ ...objectsRoot }]
  let selectedNodeID = readmeScreenshotState?.selectedNodeID ?? ''
  let inspection: Inspection | null = (readmeScreenshotState?.inspection as Inspection) ?? null
  let watchlist: WatchlistRow[] = (readmeScreenshotState?.watchlist as WatchlistRow[]) ?? []
  let sessionTrend: SessionTrendView = (readmeScreenshotState?.sessionTrend as SessionTrendView) ?? { nodes: [], points: [] }
  let focusedTrendNodeID = readmeScreenshotState?.focusedTrendNodeID ?? ''
  let logs: DiagnosticLogEntry[] = []
  let toasts: { id: number; level: string; message: string }[] = []
  let searchQuery = readmeScreenshotState?.searchQuery ?? ''
  let searchView: AddressSpaceSearchView = (readmeScreenshotState?.searchView as AddressSpaceSearchView) ?? { query: '', results: [], status: 'Connect to an OPC UA Server to search browsed Address Space metadata.' }
  let searching = false
  let searchDebounce: ReturnType<typeof setTimeout> | null = null

  $: selectedEndpointInfo = endpoints[selectedEndpoint]
  $: selectedSecurityMode = selectedEndpointInfo?.SecurityMode?.replace('MessageSecurityMode', '').trim() || ''
  $: selectedEndpointIsSecure = selectedSecurityMode !== '' && selectedSecurityMode !== 'None'
  $: canUseUsername = selectedEndpointInfo?.UserTokenTypes?.some(token => token.includes('UserName')) ?? false
  $: if (!canUseUsername && authType === 'UserName') authType = 'Anonymous'
  $: passwordRequired = authType === 'UserName'
  $: saveConnectionLabel = editingSavedConnectionID ? 'Update this Saved Connection after successful connect' : 'Save as Saved Connection'
  $: savingRequiresName = saveConnectionOnConnect && connectionName.trim().length === 0
  $: canConnect = !!selectedEndpointInfo && !connecting && !savingConnection && !savingRequiresName && (!passwordRequired || !!password) && (!selectedEndpointIsSecure || (!!clientCertificatePath && !!clientPrivateKeyPath))
  $: visibleTree = tree.filter((_, index) => !isHidden(index))

  onMount(async () => {
    if (readmeScreenshotState) return

    logs = await GetDiagnosticLogs()
    savedConnections = await GetSavedConnections()
    watchlist = await GetWatchlist()
    sessionTrend = await GetSessionTrend(focusedTrendNodeID)
    const offInspection = EventsOn('variable-inspection-updated', (payload: Inspection | null) => {
      inspection = payload
    })
    const offWatchlist = EventsOn('watchlist-updated', (payload: WatchlistRow[]) => {
      watchlist = payload || []
      if (watchlist.length > 100) {
        addToast('info', 'Watchlist has more than 100 Variable Nodes. Consider removing nodes you no longer need.')
      }
    })
    const offTrend = EventsOn('session-trend-updated', async () => {
      await refreshSessionTrend()
    })
    const offLog = EventsOn('diagnostic-log-appended', (entry: DiagnosticLogEntry) => {
      logs = [...logs, entry].slice(-500)
    })
    return () => {
      offInspection()
      offWatchlist()
      offTrend()
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
    const shouldSaveConnection = saveConnectionOnConnect
    const wasEditingSavedConnection = editingSavedConnectionName !== ''
    connecting = true
    savingConnection = shouldSaveConnection
    connectionError = ''
    try {
      await Connect({
        existingName: '',
        savedConnectionID: shouldSaveConnection ? editingSavedConnectionID : '',
        name: shouldSaveConnection ? connectionName : '',
        endpoint: endpointText,
        securityPolicy: selectedEndpointInfo.SecurityPolicy,
        securityMode: selectedEndpointInfo.SecurityMode,
        authType,
        username,
        password,
        clientCertificatePath,
        clientPrivateKeyPath,
        serverThumbprint: selectedEndpointInfo.ServerThumbprint
      })
      let saveConnectionError = ''
      if (shouldSaveConnection) {
        try {
          const saved = await SaveSavedConnection({
            existingName: editingSavedConnectionName,
            savedConnectionID: editingSavedConnectionID,
            name: connectionName,
            endpoint: endpointText,
            securityPolicy: selectedEndpointInfo.SecurityPolicy,
            securityMode: selectedEndpointInfo.SecurityMode,
            authType,
            username,
            password,
            clientCertificatePath,
            clientPrivateKeyPath,
            serverThumbprint: selectedEndpointInfo.ServerThumbprint
          })
          connectionName = saved.name
          editingSavedConnectionID = saved.id
          editingSavedConnectionName = saved.name
        } catch (error) {
          saveConnectionError = String(error)
        }
      }
      connected = true
      currentConnection = endpointText
      savedConnections = await GetSavedConnections()
      tree = [{ ...objectsRoot }]
      selectedNodeID = ''
      inspection = null
      watchlist = []
      sessionTrend = { nodes: [], points: [] }
      focusedTrendNodeID = ''
      resetSearchView()
      activeTab = 'address-space'
      if (saveConnectionError) {
        connectionError = saveConnectionError
        addToast('error', `Connected, but saving the Saved Connection failed: ${saveConnectionError}`)
      } else {
        addToast('info', shouldSaveConnection ? (wasEditingSavedConnection ? 'Connected and updated Saved Connection' : 'Connected and saved Saved Connection') : 'Connected')
      }
    } catch (error) {
      connectionError = String(error)
      addToast('error', connectionError)
    } finally {
      connecting = false
      savingConnection = false
    }
  }

  function formatSavedConnectionTime(value?: string) {
    if (!value) return 'Never connected'
    const parsed = new Date(value)
    if (Number.isNaN(parsed.getTime())) return 'Never connected'
    return `Last connected ${parsed.toLocaleString()}`
  }

  function useSavedConnection(saved: SavedConnection) {
    editingSavedConnectionID = saved.id
    editingSavedConnectionName = saved.name
    connectionName = saved.name
    endpointText = saved.endpoint
    endpoints = [{
      URL: saved.endpoint,
      SecurityPolicy: saved.securityPolicy,
      SecurityMode: saved.securityMode,
      SecurityLevel: 0,
      UserTokenTypes: saved.authType === 'UserName' ? ['UserName'] : ['Anonymous'],
      ServerThumbprint: saved.serverCertificateThumbprint
    }]
    selectedEndpoint = 0
    authType = saved.authType === 'UserName' ? 'UserName' : 'Anonymous'
    username = saved.username || ''
    password = ''
    clientCertificatePath = saved.clientCertificatePath || ''
    clientPrivateKeyPath = saved.clientPrivateKeyPath || ''
    saveConnectionOnConnect = true
    connectionError = ''
    addToast('info', 'Saved Connection details populated. Press Connect to reconnect, or uncheck update for a manual one-off connection.')
  }

  async function deleteSavedConnection(saved: SavedConnection, event: MouseEvent) {
    event.stopPropagation()
    if (!window.confirm(`Delete Saved Connection "${saved.name}"? This cannot be undone.`)) return
    deletingSavedConnectionID = saved.id
    connectionError = ''
    try {
      await DeleteSavedConnection(saved.id)
      savedConnections = await GetSavedConnections()
      if (editingSavedConnectionID === saved.id) {
        editingSavedConnectionID = ''
        editingSavedConnectionName = ''
        connectionName = ''
        saveConnectionOnConnect = false
      }
      addToast('info', 'Saved Connection deleted')
    } catch (error) {
      connectionError = String(error)
      addToast('error', connectionError)
    } finally {
      deletingSavedConnectionID = ''
    }
  }

  async function pickClientCertificate() {
    try {
      const path = await PickClientCertificate()
      if (path) clientCertificatePath = path
    } catch (error) {
      addToast('error', String(error))
    }
  }

  async function pickClientPrivateKey() {
    try {
      const path = await PickClientPrivateKey()
      if (path) clientPrivateKeyPath = path
    } catch (error) {
      addToast('error', String(error))
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
      watchlist = []
      sessionTrend = { nodes: [], points: [] }
      focusedTrendNodeID = ''
      resetSearchView()
      activeTab = 'connections'
      addToast('info', 'Disconnected')
    } catch (error) {
      addToast('error', String(error))
    }
  }

  async function toggleNode(item: TreeNode) {
    const index = tree.findIndex(entry => entry.key === item.key)
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
      const childNodes: TreeNode[] = children.map((child, childIndex) => ({ key: `${item.key}/${child.NodeID || child.BrowseName || child.DisplayName}:${childIndex}`, node: child, depth: item.depth + 1, expanded: false, childrenLoaded: false, loading: false, error: '' }))
      const end = subtreeEnd(index)
      tree = [...tree.slice(0, index + 1), ...childNodes, ...tree.slice(end)]
      tree[index].expanded = true
      tree[index].childrenLoaded = true
      tree[index].loading = false
      if (childNodes.length === 0) {
        tree[index].error = 'No child nodes'
      }
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

  function resetSearchView() {
    searchQuery = ''
    searchView = { query: '', results: [], status: connected ? 'Enter a search term to search browsed Address Space metadata.' : 'Connect to an OPC UA Server to search browsed Address Space metadata.' }
    searching = false
    if (searchDebounce) clearTimeout(searchDebounce)
    searchDebounce = null
  }

  function queueAddressSpaceSearch() {
    if (searchDebounce) clearTimeout(searchDebounce)
    searchDebounce = setTimeout(() => {
      void runAddressSpaceSearch()
    }, 275)
  }

  async function runAddressSpaceSearch() {
    searching = true
    try {
      searchView = await SearchAddressSpace(searchQuery)
    } catch (error) {
      addToast('error', String(error))
      searchView = { query: searchQuery, results: [], status: String(error) }
    } finally {
      searching = false
    }
  }

  async function activateSearchResult(result: AddressSpaceSearchResult) {
    selectedNodeID = result.node.NodeID
    if (result.node.NodeClass === 'Variable') {
      await InspectVariableNode(result.node)
    } else {
      await ClearVariableNodeInspection()
    }
  }

  async function addResultToWatchlist(result: AddressSpaceSearchResult, event: MouseEvent) {
    event.stopPropagation()
    try {
      await WatchVariableNode(result.node)
      addToast('info', 'Added to Watchlist')
    } catch (error) {
      addToast('error', String(error))
    }
  }

  async function activateNode(item: TreeNode) {
    await selectNode(item)
    if (item.node.NodeClass !== 'Variable') {
      await toggleNode(item)
    }
  }

  async function addSelectedToWatchlist() {
    if (!inspection) return
    try {
      await WatchVariableNode(inspection.node)
      addToast('info', 'Added to Watchlist')
    } catch (error) {
      addToast('error', String(error))
    }
  }

  async function removeFromWatchlist(nodeID: string) {
    try {
      await UnwatchVariableNode(nodeID)
      addToast('info', 'Removed from Watchlist')
    } catch (error) {
      addToast('error', String(error))
    }
  }

  async function inspectWatchlistRow(row: WatchlistRow) {
    selectedNodeID = row.node.NodeID
    await InspectVariableNode(row.node)
    activeTab = 'address-space'
  }

  async function openSessionTrend() {
    if (!focusedTrendNodeID && inspection?.node?.NodeID) {
      focusedTrendNodeID = inspection.node.NodeID
    }
    activeTab = 'session-trend'
    await refreshSessionTrend()
  }

  async function focusTrendNode(nodeID: string) {
    focusedTrendNodeID = nodeID
    await refreshSessionTrend()
  }

  async function refreshSessionTrend() {
    sessionTrend = await GetSessionTrend(focusedTrendNodeID)
    const focusedNodeIsObserved = sessionTrend.nodes.some(node => node.node.NodeID === focusedTrendNodeID)
    if ((!focusedTrendNodeID || !focusedNodeIsObserved) && sessionTrend.nodes.length > 0) {
      focusedTrendNodeID = sessionTrend.nodes[0].node.NodeID
      sessionTrend = await GetSessionTrend(focusedTrendNodeID)
    }
  }

  function isHidden(index: number) {
    let depth = tree[index].depth
    for (let cursor = index - 1; cursor >= 0 && depth > 0; cursor--) {
      const ancestor = tree[cursor]
      if (ancestor.depth < depth) {
        if (!ancestor.expanded) return true
        depth = ancestor.depth
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

  function compactDateTime(value: string) {
    if (!value || value.startsWith('0001-')) return '—'
    return new Date(value).toLocaleString()
  }

  function nodeIcon(nodeClass: string) {
    if (nodeClass === 'Variable') return 'monitoring'
    if (nodeClass === 'Object') return 'account_tree'
    return 'schema'
  }

  function isWatched(nodeID: string) {
    return watchlist.some(row => row.node.NodeID === nodeID)
  }

  function statusBucket(row: WatchlistRow) {
    const status = row.value?.Status || ''
    if (row.stale) return 'Stale'
    if (status.includes('Bad') || row.error) return 'Bad'
    if (status.includes('Uncertain')) return 'Uncertain'
    if (status.includes('Good')) return 'Good'
    return 'Waiting'
  }

  function statusDotClass(row: WatchlistRow) {
    const bucket = statusBucket(row)
    if (bucket === 'Good') return 'bg-emerald-400 shadow-[0_0_8px_rgba(74,222,128,0.4)]'
    if (bucket === 'Bad') return 'bg-error shadow-[0_0_8px_rgba(255,180,171,0.5)] animate-pulse'
    if (bucket === 'Uncertain' || bucket === 'Stale') return 'bg-tertiary-container shadow-[0_0_8px_rgba(241,160,43,0.4)]'
    return 'bg-outline'
  }

  function statusTextClass(row: WatchlistRow) {
    const bucket = statusBucket(row)
    if (bucket === 'Bad') return 'text-error'
    if (bucket === 'Uncertain' || bucket === 'Stale' || row.outOfRange) return 'text-tertiary'
    if (bucket === 'Good') return 'text-primary-fixed-dim'
    return 'text-on-surface-variant'
  }

  $: watchlistCounts = watchlist.reduce(
    (counts, row) => {
      const bucket = statusBucket(row)
      if (bucket === 'Good') counts.good++
      else if (bucket === 'Bad') counts.bad++
      else if (bucket === 'Uncertain') counts.uncertain++
      else if (bucket === 'Stale') counts.stale++
      if (row.outOfRange) counts.outOfRange++
      return counts
    },
    { good: 0, bad: 0, uncertain: 0, stale: 0, outOfRange: 0 }
  )

  function navButtonClass(active: boolean) {
    return active
      ? 'flex w-full items-center gap-md rounded border-l-2 px-md py-sm text-left transition-colors bg-primary-container text-background border-primary-container font-bold'
      : 'flex w-full items-center gap-md rounded border-l-2 px-md py-sm text-left transition-colors text-on-surface-variant border-transparent hover:bg-surface-container-highest'
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
      <button class={navButtonClass(activeTab === 'connections')} on:click={() => (activeTab = 'connections')}>
        <span class="material-symbols-outlined">settings_input_component</span><span class="label text-current">Connection Manager</span>
      </button>
      <button class={navButtonClass(activeTab === 'address-space')} on:click={() => (activeTab = 'address-space')}>
        <span class="material-symbols-outlined">account_tree</span><span class="label text-current">Address Space</span>
      </button>
      <button class={navButtonClass(activeTab === 'watchlist')} on:click={() => (activeTab = 'watchlist')}>
        <span class="material-symbols-outlined">analytics</span><span class="label text-current">Watchlist</span>
      </button>
      <button class={navButtonClass(activeTab === 'session-trend')} on:click={openSessionTrend}>
        <span class="material-symbols-outlined">show_chart</span><span class="label text-current">Session Trend</span>
      </button>
    </nav>

    <div class="space-y-sm border-t border-outline-variant px-md pt-md">
      <button class={navButtonClass(activeTab === 'logs')} on:click={() => (activeTab = 'logs')}>
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
          <input class="w-full bg-transparent text-sm outline-none placeholder:text-on-surface-variant" placeholder="Search Address Space..." bind:value={searchQuery} on:input={queueAddressSpaceSearch} on:focus={() => (activeTab = 'address-space')} />
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
            <p class="mt-sm text-on-surface-variant">Create a Saved Connection from non-secret details, then reconnect from it after restarting OPC UA Studio.</p>
          </div>

          <div class="panel overflow-hidden">
            <div class="flex items-center justify-between border-b border-outline-variant p-md">
              <div><p class="label">Saved Connections</p><h3 class="text-xl font-semibold">Reconnect details</h3></div>
              <span class="rounded bg-surface-container-highest px-sm py-xs font-mono text-xs text-on-surface-variant">{savedConnections.length}</span>
            </div>
            {#if savedConnections.length === 0}
              <div class="space-y-md p-lg text-on-surface-variant">
                <p>No Saved Connections yet.</p>
                <p>Use the connection form below and check Save as Saved Connection before connecting.</p>
                <p>Leave it unchecked to make a manual one-off connection without saving.</p>
              </div>
            {:else}
              <div class="divide-y divide-outline-variant">
                {#each savedConnections as saved (saved.id)}
                  <div role="button" tabindex="0" class="block w-full cursor-pointer p-md text-left transition-colors hover:bg-surface-container-high" on:click={() => useSavedConnection(saved)} on:keydown={(event) => event.key === 'Enter' && useSavedConnection(saved)}>
                    <div class="flex items-center justify-between gap-md">
                      <span class="font-semibold text-on-surface">{saved.name}</span>
                      <span class="rounded bg-surface-container-highest px-sm py-xs font-mono text-xs text-primary">{saved.authType}</span>
                    </div>
                    <p class="mt-xs truncate font-mono text-sm text-on-surface-variant">{saved.endpoint}</p>
                    <p class="mt-xs truncate text-xs text-on-surface-variant">{saved.securityPolicy || 'None'} / {saved.securityMode || 'None'}{saved.username ? ` • ${saved.username}` : ''}</p>
                    {#if saved.serverCertificateThumbprint}<p class="mt-xs truncate font-mono text-xs text-on-surface-variant">Server certificate thumbprint: {saved.serverCertificateThumbprint}</p>{/if}
                    <div class="mt-xs flex items-center justify-between gap-md text-xs text-on-surface-variant">
                      <span>{formatSavedConnectionTime(saved.lastConnectedAt)}</span>
                      <button class="rounded p-xs text-error hover:bg-error-container/20" disabled={deletingSavedConnectionID === saved.id} on:click={(event) => deleteSavedConnection(saved, event)} title="Delete Saved Connection"><span class="material-symbols-outlined text-[18px]">delete</span></button>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
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
                      {#if endpoint.ServerThumbprint}<p class="mt-xs truncate font-mono text-xs text-on-surface-variant">Server certificate thumbprint: {endpoint.ServerThumbprint}</p>{/if}
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
                {#if selectedEndpointIsSecure}
                  <div class="space-y-sm rounded border border-outline-variant bg-surface-container-low p-md">
                    <p class="label">Client Certificate</p>
                    <p class="text-sm text-on-surface-variant">Secure endpoints require a PEM/CRT client certificate and PEM/KEY private key.</p>
                    <label class="block space-y-xs">
                      <span class="label">Certificate Path</span>
                      <div class="flex gap-sm">
                        <input class="field min-w-0 flex-1" bind:value={clientCertificatePath} placeholder="C:\\certs\\client.crt" />
                        <button class="btn-secondary shrink-0" on:click={pickClientCertificate}>Browse…</button>
                      </div>
                    </label>
                    <label class="block space-y-xs">
                      <span class="label">Private Key Path</span>
                      <div class="flex gap-sm">
                        <input class="field min-w-0 flex-1" bind:value={clientPrivateKeyPath} placeholder="C:\\certs\\client.key" />
                        <button class="btn-secondary shrink-0" on:click={pickClientPrivateKey}>Browse…</button>
                      </div>
                    </label>
                  </div>
                {/if}
                <label class="flex items-start gap-sm rounded border border-outline-variant bg-surface-container-low p-md text-sm text-on-surface">
                  <input class="mt-1" type="checkbox" bind:checked={saveConnectionOnConnect} />
                  <span>
                    <span class="font-medium">{saveConnectionLabel}</span>
                    <span class="mt-xs block text-on-surface-variant">Leave unchecked for a manual one-off connection.</span>
                  </span>
                </label>
                {#if saveConnectionOnConnect}
                  <label class="block space-y-xs">
                    <span class="label">Saved Connection Name</span>
                    <input class="field w-full" bind:value={connectionName} placeholder="Control Gateway" />
                    {#if editingSavedConnectionName}<span class="text-xs text-on-surface-variant">Updating {editingSavedConnectionName}</span>{/if}
                  </label>
                {/if}
                <button class="btn-primary w-full" on:click={connect} disabled={!canConnect}>{connecting ? 'Connecting…' : savingConnection ? 'Saving…' : 'Connect'}</button>
                {#if savingRequiresName}
                  <p class="text-sm text-tertiary">Enter a Saved Connection name to save these reconnect details.</p>
                {:else if passwordRequired && !password}
                  <p class="text-sm text-tertiary">Enter the password for this Saved Connection before connecting. Passwords are never saved.</p>
                {:else if selectedEndpointIsSecure && (!clientCertificatePath || !clientPrivateKeyPath)}
                  <p class="text-sm text-tertiary">Provide a client certificate and private key to connect to this secure endpoint.</p>
                {:else}
                  <p class="text-sm text-on-surface-variant">Client Certificate authentication and issued tokens are intentionally deferred in this slice.</p>
                {/if}
              </div>
            </div>
          {/if}
        </section>
      {:else if activeTab === 'address-space'}
        <section class="grid h-full min-h-[600px] gap-lg xl:grid-cols-[minmax(320px,0.85fr)_minmax(380px,1.05fr)_minmax(420px,1.1fr)]">
          <div class="panel flex min-h-0 flex-col overflow-hidden">
            <div class="flex items-center justify-between border-b border-outline-variant p-md">
              <div><p class="label">Address Space</p><h2 class="text-xl font-semibold">Objects</h2></div>
              {#if !connected}<span class="text-sm text-on-surface-variant">Disconnected</span>{/if}
            </div>
            <div class="min-h-0 flex-1 overflow-auto p-sm">
              {#if !connected}
                <div class="p-lg text-on-surface-variant">Connect to an OPC UA Server to browse its Address Space.</div>
              {:else}
                {#each visibleTree as item (item.key)}
                  <div class="group flex items-center gap-xs rounded px-sm py-xs hover:bg-surface-container-high" style={`padding-left: ${8 + item.depth * 18}px`}>
                    <button class="flex h-6 w-6 items-center justify-center rounded hover:bg-surface-container-highest" on:click={() => toggleNode(item)} title="Expand or collapse">
                      {#if item.loading}<span class="text-xs text-primary">…</span>{:else}<span class="material-symbols-outlined text-[18px]">{item.expanded ? 'expand_more' : 'chevron_right'}</span>{/if}
                    </button>
                    <button class="min-w-0 flex-1 truncate rounded px-xs py-xs text-left {selectedNodeID === item.node.NodeID ? 'bg-secondary-container text-on-secondary-container' : 'text-on-surface'}" on:click={() => activateNode(item)}>
                      <span class="mr-sm font-medium">{item.node.DisplayName}</span><span class="font-mono text-xs text-on-surface-variant">{item.node.NodeClass}</span>
                    </button>
                  </div>
                  {#if item.error}<div class="ml-lg text-xs text-error">{item.error}</div>{/if}
                {/each}
              {/if}
            </div>
          </div>

          <div class="panel flex min-h-0 flex-col overflow-hidden">
            <div class="border-b border-outline-variant p-md">
              <p class="label">Search</p>
              <h2 class="text-xl font-semibold">Address Space Search</h2>
              <p class="mt-xs text-sm text-on-surface-variant">Search browsed metadata: DisplayName, BrowseName, NodeID, and NodeClass.</p>
              <div class="relative mt-md">
                <span class="material-symbols-outlined absolute left-md top-1/2 -translate-y-1/2 text-on-surface-variant">search</span>
                <input class="field w-full py-md pl-[48px] text-base" placeholder="Search Address Space…" bind:value={searchQuery} on:input={queueAddressSpaceSearch} />
              </div>
            </div>
            <div class="min-h-0 flex-1 overflow-auto p-md">
              {#if searching}
                <div class="mb-md text-sm text-primary">Searching…</div>
              {/if}
              {#if searchView.results.length === 0}
                <div class="flex min-h-[260px] items-center justify-center rounded border border-dashed border-outline-variant bg-surface-container-low p-lg text-center text-on-surface-variant">
                  <div>
                    <span class="material-symbols-outlined text-3xl text-primary">manage_search</span>
                    <p class="mt-sm">{searchView.status}</p>
                    {#if connected}<p class="mt-xs text-xs">Browse the tree to add more Address Space metadata to Search.</p>{/if}
                  </div>
                </div>
              {:else}
                <div class="space-y-md">
                  {#each searchView.results as result (result.node.NodeID)}
                    <div role="button" tabindex="0" class="group relative block w-full cursor-pointer overflow-hidden rounded-lg border border-outline-variant bg-surface-container p-md text-left transition-colors hover:border-primary/70 {selectedNodeID === result.node.NodeID ? 'border-primary bg-secondary-container/40' : ''}" on:click={() => activateSearchResult(result)} on:keydown={(event) => event.key === 'Enter' && activateSearchResult(result)}>
                      <div class="absolute left-0 top-0 bottom-0 w-[2px] bg-primary {selectedNodeID === result.node.NodeID ? 'scale-y-100' : 'scale-y-0 group-hover:scale-y-100'} origin-top transition-transform"></div>
                      <div class="flex items-start justify-between gap-md">
                        <div class="flex min-w-0 gap-sm">
                          <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded border border-outline-variant bg-surface-container-high text-primary">
                            <span class="material-symbols-outlined">{nodeIcon(result.node.NodeClass)}</span>
                          </div>
                          <div class="min-w-0">
                            <h3 class="truncate text-lg font-semibold text-on-surface">{result.node.DisplayName}</h3>
                            <p class="mt-xs truncate font-mono text-xs text-on-surface-variant">{result.node.NodeID}</p>
                          </div>
                        </div>
                        <span class="rounded bg-surface-container-highest px-sm py-xs font-mono text-xs text-primary">{result.node.NodeClass}</span>
                      </div>
                      <div class="mt-md grid gap-sm text-xs lg:grid-cols-2">
                        <div class="rounded border border-outline-variant/60 bg-surface p-sm"><span class="label block">BrowseName</span><span class="font-mono text-on-surface">{result.node.BrowseName || '—'}</span></div>
                        <div class="rounded border border-outline-variant/60 bg-surface p-sm"><span class="label block">Match</span><span class="font-mono text-on-surface">{result.matchKind}: {result.matchText}</span></div>
                      </div>
                      <div class="mt-md flex items-center justify-between gap-sm">
                        <span class="rounded bg-surface-container-high px-sm py-xs font-mono text-[10px] text-on-surface-variant">{result.source}</span>
                        {#if result.node.NodeClass === 'Variable'}
                          <button class="btn-primary shrink-0 px-sm py-xs text-xs" disabled={isWatched(result.node.NodeID)} on:click={(event) => addResultToWatchlist(result, event)}>{isWatched(result.node.NodeID) ? 'In Watchlist' : 'Add to Watchlist'}</button>
                        {/if}
                      </div>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          </div>

          <div class="panel flex min-h-0 flex-col overflow-hidden">
            <div class="flex items-center justify-between gap-md border-b border-outline-variant p-md">
              <div class="min-w-0"><p class="label">Variable Node Inspection</p><h2 class="truncate text-xl font-semibold">{inspection?.node?.DisplayName ?? 'No Variable Node selected'}</h2></div>
              {#if inspection}
                {#if inspection.watched}
                  <button class="btn-secondary shrink-0" on:click={() => removeFromWatchlist(inspection?.node.NodeID || '')}>Remove from Watchlist</button>
                {:else}
                  <button class="btn-primary shrink-0" on:click={addSelectedToWatchlist}>Add to Watchlist</button>
                {/if}
              {/if}
            </div>
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
      {:else if activeTab === 'watchlist'}
        <section class="space-y-lg">
          <div class="flex items-end justify-between gap-md">
            <div>
              <p class="label">Watchlist</p>
              <h2 class="text-3xl font-semibold">Watched Variable Nodes</h2>
              <p class="mt-sm text-on-surface-variant">Live Values from Variable Nodes selected during this Troubleshooting Session.</p>
            </div>
          </div>

          {#if !connected}
            <div class="panel flex min-h-[420px] items-center justify-center p-xl text-center text-on-surface-variant">Connect to an OPC UA Server to create a Watchlist.</div>
          {:else if watchlist.length === 0}
            <div class="panel flex min-h-[420px] items-center justify-center p-xl text-center text-on-surface-variant">No watched Variable Nodes. Select a Variable Node in the Address Space and choose Add to Watchlist.</div>
          {:else}
            <div class="panel overflow-hidden">
              <div class="grid grid-cols-12 gap-sm border-b border-outline-variant bg-surface-container-highest px-md py-sm text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                <div class="col-span-1 pl-xs">Status</div>
                <div class="col-span-3">Variable Node</div>
                <div class="col-span-3">NodeId</div>
                <div class="col-span-2 text-right">Live Value</div>
                <div class="col-span-1 text-center">Data Type</div>
                <div class="col-span-2 text-right pr-xs">Source Timestamp</div>
              </div>
              <div class="max-h-[62vh] overflow-auto font-mono text-sm">
                {#each watchlist as row (row.node.NodeID)}
                  <div class="grid grid-cols-12 items-center gap-sm border-b border-outline-variant px-md py-sm transition-colors hover:bg-surface-container-high {statusBucket(row) === 'Bad' ? 'bg-error/5' : ''}">
                    <button class="col-span-1 flex items-center gap-sm pl-xs text-left" on:click={() => inspectWatchlistRow(row)} title={statusBucket(row)}>
                      <span class="h-2 w-2 rounded-full {statusDotClass(row)}"></span>
                      <span class="sr-only">{statusBucket(row)}</span>
                    </button>
                    <button class="col-span-3 truncate text-left hover:text-primary {statusTextClass(row)}" on:click={() => inspectWatchlistRow(row)}>{row.node.DisplayName}</button>
                    <button class="col-span-3 truncate text-left text-on-surface-variant" on:click={() => inspectWatchlistRow(row)}>{row.node.NodeID}</button>
                    <button class="col-span-2 truncate text-right font-bold {statusTextClass(row)}" on:click={() => inspectWatchlistRow(row)}>{row.error || row.value?.Value || '—'}{row.engineeringUnit ? ` ${row.engineeringUnit}` : ''}</button>
                    <button class="col-span-1 mx-auto w-max rounded border border-outline-variant px-xs text-center text-xs text-on-surface-variant" on:click={() => inspectWatchlistRow(row)}>{row.dataType || '—'}</button>
                    <div class="col-span-2 flex items-center justify-end gap-sm pr-xs text-on-surface-variant">
                      <button class="truncate text-right" on:click={() => inspectWatchlistRow(row)}>{compactTime(row.value?.SourceTimestamp)}</button>
                      {#if row.outOfRange}<span class="rounded bg-tertiary-container/20 px-xs text-xs text-tertiary" title={row.outOfRange}>Out-of-Range</span>{/if}
                      {#if row.stale}<span class="rounded bg-tertiary-container/20 px-xs text-xs text-tertiary">Stale</span>{/if}
                      <button class="rounded p-xs hover:bg-surface-container-highest" on:click={() => removeFromWatchlist(row.node.NodeID)} title="Remove from Watchlist"><span class="material-symbols-outlined text-[18px]">close</span></button>
                    </div>
                  </div>
                {/each}
              </div>
              <div class="flex items-center justify-between border-t border-outline-variant bg-surface-container px-md py-sm text-xs font-semibold text-on-surface-variant">
                <span>Showing {watchlist.length} watched Variable Nodes</span>
                <div class="flex items-center gap-sm">
                  <span><span class="mr-xs inline-block h-2 w-2 rounded-full bg-emerald-400"></span>{watchlistCounts.good} Good</span>
                  <span><span class="mr-xs inline-block h-2 w-2 rounded-full bg-error"></span>{watchlistCounts.bad} Bad</span>
                  <span><span class="mr-xs inline-block h-2 w-2 rounded-full bg-tertiary-container"></span>{watchlistCounts.uncertain} Uncertain</span>
                  <span>{watchlistCounts.stale} Stale</span>
                  <span>{watchlistCounts.outOfRange} Out-of-Range</span>
                </div>
              </div>
            </div>
          {/if}
        </section>
      {:else if activeTab === 'session-trend'}
        <section class="grid h-full min-h-[600px] gap-lg xl:grid-cols-[360px_1fr]">
          <div class="panel flex min-h-0 flex-col overflow-hidden">
            <div class="border-b border-outline-variant p-md">
              <p class="label">Session Trend</p>
              <h2 class="text-xl font-semibold">Observed Variable Nodes</h2>
              <p class="mt-xs text-sm text-on-surface-variant">Temporary Live Value history for this Troubleshooting Session.</p>
            </div>
            <div class="min-h-0 flex-1 overflow-auto p-sm">
              {#if !connected}
                <div class="p-lg text-on-surface-variant">Connect to an OPC UA Server to collect Session Trend history.</div>
              {:else if sessionTrend.nodes.length === 0}
                <div class="p-lg text-on-surface-variant">Inspect or watch a Variable Node and wait for Live Value updates.</div>
              {:else}
                {#each sessionTrend.nodes as trendNode (trendNode.node.NodeID)}
                  <button class="mb-xs block w-full rounded p-sm text-left transition-colors hover:bg-surface-container-high {focusedTrendNodeID === trendNode.node.NodeID ? 'bg-secondary-container text-on-secondary-container' : ''}" on:click={() => focusTrendNode(trendNode.node.NodeID)}>
                    <div class="flex items-center justify-between gap-sm">
                      <span class="truncate font-medium">{trendNode.node.DisplayName}</span>
                      <span class="rounded bg-surface-container-highest px-xs font-mono text-xs text-primary">{trendNode.pointCount}</span>
                    </div>
                    <p class="mt-xs truncate font-mono text-xs text-on-surface-variant">{trendNode.node.NodeID}</p>
                    <div class="mt-xs flex items-center justify-between gap-sm font-mono text-xs">
                      <span class="truncate text-primary-fixed-dim">{trendNode.latestValue || '—'}</span>
                      <span class="truncate text-on-surface-variant">{trendNode.status || 'Waiting'}</span>
                    </div>
                  </button>
                {/each}
              {/if}
            </div>
          </div>

          <div class="panel flex min-h-0 flex-col overflow-hidden">
            <div class="flex items-center justify-between gap-md border-b border-outline-variant p-md">
              <div class="min-w-0">
                <p class="label">Focused Variable Node</p>
                <h2 class="truncate text-xl font-semibold">{sessionTrend.nodes.find(node => node.node.NodeID === focusedTrendNodeID)?.node.DisplayName ?? 'No Observed Variable Node selected'}</h2>
                {#if focusedTrendNodeID}<p class="mt-xs truncate font-mono text-xs text-on-surface-variant">{focusedTrendNodeID}</p>{/if}
              </div>
              <span class="shrink-0 rounded bg-surface-container-highest px-sm py-xs font-mono text-xs text-on-surface-variant">Latest {sessionTrend.points.length} updates</span>
            </div>
            <div class="min-h-0 flex-1 overflow-auto">
              {#if !connected}
                <div class="flex h-full items-center justify-center p-xl text-center text-on-surface-variant">Session Trend history is available during a connected Troubleshooting Session.</div>
              {:else if !focusedTrendNodeID || sessionTrend.points.length === 0}
                <div class="flex h-full items-center justify-center p-xl text-center text-on-surface-variant">Waiting for Live Value updates for the focused Observed Variable Node.</div>
              {:else}
                <div class="grid grid-cols-12 gap-sm border-b border-outline-variant bg-surface-container-highest px-md py-sm text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                  <div class="col-span-2">Time</div>
                  <div class="col-span-2 text-right">Value</div>
                  <div class="col-span-2">Status</div>
                  <div class="col-span-2">Source Timestamp</div>
                  <div class="col-span-2">Server Timestamp</div>
                  <div class="col-span-2">Receive Time</div>
                </div>
                <div class="font-mono text-xs">
                  {#each sessionTrend.points as point, index (`${point.receivedAt}-${index}`)}
                    <div class="grid grid-cols-12 items-center gap-sm border-b border-outline-variant px-md py-sm hover:bg-surface-container-high">
                      <div class="col-span-2 truncate" title={point.timestamp}>{compactDateTime(point.timestamp)}</div>
                      <div class="col-span-2 truncate text-right font-bold text-primary" title={point.value}>{point.value || '—'}</div>
                      <div class="col-span-2 truncate {point.status?.includes('Bad') ? 'text-error' : point.status?.includes('Uncertain') ? 'text-tertiary' : 'text-on-surface'}" title={point.status}>{point.status || '—'}</div>
                      <div class="col-span-2 truncate text-on-surface-variant" title={point.sourceTimestamp}>{compactDateTime(point.sourceTimestamp)}</div>
                      <div class="col-span-2 truncate text-on-surface-variant" title={point.serverTimestamp}>{compactDateTime(point.serverTimestamp)}</div>
                      <div class="col-span-2 truncate text-on-surface-variant" title={point.receivedAt}>{compactDateTime(point.receivedAt)}</div>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
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
