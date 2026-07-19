<script lang="ts">
  import { onMount } from 'svelte'
  import { BrowseChildren, CallMethod, ClearVariableNodeInspection, Connect, DeleteSavedConnection, Disconnect, DiscoverEndpoints, GetDiagnosticLogs, GetMethodDetails, GetSavedConnections, GetSessionSafety, GetSessionTrend, GetWatchlist, InspectVariableNode, PickClientCertificate, PickClientPrivateKey, RefreshVariableNodeValue, SaveSavedConnection, SearchAddressSpace, SetReadOnlyMode, UnwatchVariableNode, WatchVariableNode, WriteVariableNodeValue } from '../wailsjs/go/main/App.js'
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
    ParentNodeID: string
    NodeID: string
    DisplayName: string
    BrowseName: string
    NodeClass: string
  }

  type MethodArgument = {
    Name: string
    DataType: string
    DataTypeID: string
    ValueRank: string
    Description: string
    ArrayDimensions: number[]
    Supported: boolean
  }

  type MethodDetails = {
    ObjectNodeID: string
    MethodNodeID: string
    Description: string
    Executable: boolean
    UserExecutable: boolean
    InputArguments: MethodArgument[]
    OutputArguments: MethodArgument[]
  }

  type MethodCallResult = {
    StatusCode: string
    InputArgumentResults: string[]
    OutputArguments: Array<{ DataType: string; Value: string }>
  }

  type MethodCallAvailability = {
    selectedMethod: AddressNode | null
    details: MethodDetails | null
    detailsLoading: boolean
    detailsError: string
    inputs: string[]
    submitting: boolean
    connected: boolean
    readOnlyMode: boolean
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
      UserAccessLevel: string
      UserAccessLevelAvailable: boolean
      Writable: boolean
      WriteAvailability: string
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
  type SessionSafety = { connected: boolean; readOnlyMode: boolean }

  type VariableNodeWriteResult = {
    nodeID: string
    targetValue: string
    status: string
    readBack: { Value: string; Status: string; SourceTimestamp: string; ServerTimestamp: string }
    warning: string
  }

  type WriteConfirmationSnapshot = {
    nodeID: string
    updateCount: number
    value: string
    status: string
    sourceTimestamp: string
    serverTimestamp: string
  }

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
    node: { ParentNodeID: '', NodeID: 'i=85', DisplayName: 'Objects', BrowseName: 'Objects', NodeClass: 'Object' },
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
  let readOnlyMode = readmeScreenshotState?.readOnlyMode ?? true
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
  let logs: DiagnosticLogEntry[] = (readmeScreenshotState?.logs as DiagnosticLogEntry[]) ?? []
  let toasts: { id: number; level: string; message: string }[] = []
  let searchQuery = readmeScreenshotState?.searchQuery ?? ''
  let searchView: AddressSpaceSearchView = (readmeScreenshotState?.searchView as AddressSpaceSearchView) ?? { query: '', results: [], status: 'Connect to an OPC UA Server to search browsed Address Space metadata.' }
  let searching = false
  let searchDebounce: ReturnType<typeof setTimeout> | null = null
  let refreshingNodeID = ''
  let writeTargetValue = ''
  let writeSubmitting = false
  let writeResult: VariableNodeWriteResult | null = null
  let writeError = ''
  let writeConfirmOpen = false
  let writeConfirmationSnapshot: WriteConfirmationSnapshot | null = null
  let selectedMethod: AddressNode | null = null
  let methodDetails: MethodDetails | null = null
  let methodDetailsLoading = false
  let methodDetailsError = ''
  let methodInputs: string[] = []
  let methodSubmitting = false
  let methodResult: MethodCallResult | null = null
  let methodCallError = ''

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
  $: writeDisabledReasons = variableNodeWriteDisabledReasons(inspection, connected, readOnlyMode, writeTargetValue)
  $: canOpenWriteConfirmation = !!inspection && writeDisabledReasons.length === 0 && !writeSubmitting
  $: writeConfirmationInvalidated = isWriteConfirmationInvalidated(inspection, writeConfirmationSnapshot)
  $: writeRangeWarning = writeTargetRangeWarning(inspection, writeTargetValue)
  $: writeStatusWarning = inspection && !inspection.stale && inspection.value?.Status && !inspection.value.Status.includes('Good') ? `Current status is ${inspection.value.Status}; confirm the value is safe to change.` : ''
  $: methodDisabledReasons = methodCallDisabledReasons({ selectedMethod, details: methodDetails, detailsLoading: methodDetailsLoading, detailsError: methodDetailsError, inputs: methodInputs, submitting: methodSubmitting, connected, readOnlyMode })
  $: canCallMethod = !!selectedMethod && !!methodDetails && methodDisabledReasons.length === 0

  onMount(() => {
    let disposed = false
    let eventUnsubscribers: Array<() => void> = []

    async function initialize() {
      if (!readmeScreenshotState) {
        logs = await GetDiagnosticLogs()
        savedConnections = await GetSavedConnections()
        const sessionSafety = await GetSessionSafety()
        applySessionSafety(sessionSafety)
        watchlist = await GetWatchlist()
        sessionTrend = await GetSessionTrend(focusedTrendNodeID)
      } else if (!readmeScreenshotState.receiveRuntimeEvents) {
        return
      }

      if (disposed) return
      eventUnsubscribers = [
        EventsOn('variable-inspection-updated', (payload: Inspection | null) => {
          const previousNodeID = inspection?.node?.NodeID || ''
          inspection = payload
          if (previousNodeID && payload?.node?.NodeID !== previousNodeID) resetWriteState()
        }),
        EventsOn('watchlist-updated', (payload: WatchlistRow[]) => {
          watchlist = payload || []
          if (watchlist.length > 100) {
            addToast('info', 'Watchlist has more than 100 Variable Nodes. Consider removing nodes you no longer need.')
          }
        }),
        EventsOn('session-trend-updated', async () => {
          await refreshSessionTrend()
        }),
        EventsOn('diagnostic-log-appended', (entry: DiagnosticLogEntry) => {
          logs = [...logs, entry].slice(-500)
        }),
        EventsOn('session-safety-updated', (payload: SessionSafety) => {
          applySessionSafety(payload)
        })
      ]
    }

    void initialize()
    return () => {
      disposed = true
      eventUnsubscribers.forEach(unsubscribe => unsubscribe())
    }
  })

  function applySessionSafety(sessionSafety: SessionSafety) {
    connected = sessionSafety.connected
    readOnlyMode = sessionSafety.readOnlyMode
    if (!sessionSafety.connected) resetMethodState()
  }

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
      const sessionSafety = await GetSessionSafety()
      applySessionSafety(sessionSafety)
      currentConnection = endpointText
      savedConnections = await GetSavedConnections()
      tree = [{ ...objectsRoot }]
      selectedNodeID = ''
      inspection = null
      resetMethodState()
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
      ServerThumbprint: saved.serverCertificateThumbprint || ''
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
      const sessionSafety = await GetSessionSafety()
      applySessionSafety(sessionSafety)
      currentConnection = ''
      tree = [{ ...objectsRoot }]
      selectedNodeID = ''
      inspection = null
      resetMethodState()
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

  async function setReadOnlyMode(enabled: boolean) {
    if (!enabled && !window.confirm('Allow changes to the OPC UA Server?\n\nThis enables Variable Node Writes and Method calls until you disconnect or turn Read-Only Mode back on. Variable Node Writes still require confirmation.')) return
    try {
      await SetReadOnlyMode(enabled)
      const sessionSafety = await GetSessionSafety()
      applySessionSafety(sessionSafety)
      addToast('info', enabled ? 'Read-Only Mode enabled' : 'Changes allowed for this session')
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
    selectedNodeID = nodeSelectionKey(item.node)
    if (item.node.NodeClass === 'Variable') {
      resetMethodState()
      await InspectVariableNode(item.node)
    } else if (item.node.NodeClass === 'Method') {
      await activateMethod(item.node)
    } else {
      resetMethodState()
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
    selectedNodeID = nodeSelectionKey(result.node)
    if (result.node.NodeClass === 'Variable') {
      resetMethodState()
      await InspectVariableNode(result.node)
    } else if (result.node.NodeClass === 'Method') {
      await activateMethod(result.node)
    } else {
      resetMethodState()
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
    if (item.node.NodeClass !== 'Variable' && item.node.NodeClass !== 'Method') {
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

  async function refreshInspectionValue() {
    if (!inspection || refreshingNodeID) return
    refreshingNodeID = inspection.node.NodeID
    try {
      await RefreshVariableNodeValue(inspection.node.NodeID)
      addToast('info', 'Live Value refreshed')
    } catch (error) {
      addToast('error', `Refresh failed: ${String(error)}`)
    } finally {
      refreshingNodeID = ''
    }
  }

  function resetWriteState() {
    writeTargetValue = ''
    writeSubmitting = false
    writeResult = null
    writeError = ''
    writeConfirmOpen = false
    writeConfirmationSnapshot = null
  }

  function nodeSelectionKey(node: AddressNode) {
    return node.NodeClass === 'Method' ? `${node.ParentNodeID || ''}\u0000${node.NodeID}` : node.NodeID
  }

  function resetMethodState() {
    selectedMethod = null
    methodDetails = null
    methodDetailsLoading = false
    methodDetailsError = ''
    methodInputs = []
    methodSubmitting = false
    methodResult = null
    methodCallError = ''
  }

  async function activateMethod(node: AddressNode) {
    resetMethodState()
    resetWriteState()
    inspection = null
    selectedMethod = node
    methodDetailsLoading = true
    const selectionKey = nodeSelectionKey(node)
    await ClearVariableNodeInspection()
    try {
      const details = await GetMethodDetails({ objectNodeID: node.ParentNodeID || '', methodNodeID: node.NodeID })
      if (selectedMethod && nodeSelectionKey(selectedMethod) === selectionKey) {
        methodDetails = { ...details, InputArguments: details.InputArguments || [], OutputArguments: details.OutputArguments || [] }
        methodInputs = methodDetails.InputArguments.map(() => '')
      }
    } catch (error) {
      if (selectedMethod && nodeSelectionKey(selectedMethod) === selectionKey) methodDetailsError = String(error)
    } finally {
      if (selectedMethod && nodeSelectionKey(selectedMethod) === selectionKey) methodDetailsLoading = false
    }
  }

  function updateMethodInput(index: number, value: string) {
    methodInputs = methodInputs.map((current, currentIndex) => currentIndex === index ? value : current)
    methodResult = null
    methodCallError = ''
  }

  function methodCallDisabledReasons(state: MethodCallAvailability) {
    const reasons: string[] = []
    if (!state.connected) reasons.push('Connect to an OPC UA Server before calling a Method.')
    if (state.readOnlyMode) reasons.push('Read-Only Mode is active.')
    if (state.detailsLoading) reasons.push('Method metadata is loading.')
    if (state.detailsError) reasons.push(`Method metadata failed to load: ${state.detailsError}`)
    if (!state.selectedMethod) return reasons
    if (!state.details && !state.detailsLoading && !state.detailsError) reasons.push('Method metadata is unavailable.')
    if (!state.details) return reasons
    if (!state.details.Executable) reasons.push('Method is not executable.')
    else if (!state.details.UserExecutable) reasons.push('Method is not executable for the current session.')
    state.details.InputArguments.forEach((argument, index) => {
      const unsupportedType = !isSupportedWriteDataType(argument.DataType)
      const unsupportedRank = argument.ValueRank !== 'Scalar'
      if (!argument.Supported || unsupportedType || unsupportedRank) {
        const limitations = [unsupportedType ? `DataType ${argument.DataType}` : '', unsupportedRank ? `ValueRank ${argument.ValueRank}` : ''].filter(Boolean).join(' and ')
        reasons.push(`${argument.Name || `Input ${index + 1}`} uses unsupported ${limitations || 'argument metadata'}.`)
        return
      }
      const error = parseScalarInputError(argument.DataType, state.inputs[index] || '', argument.Name || `Input ${index + 1}`)
      if (error) reasons.push(error)
    })
    if (state.submitting) reasons.push('Method call is in progress.')
    return reasons
  }

  async function submitMethodCall() {
    if (!selectedMethod || !methodDetails || !canCallMethod || methodSubmitting) return
    const selectionKey = nodeSelectionKey(selectedMethod)
    methodSubmitting = true
    methodResult = null
    methodCallError = ''
    try {
      const result = await CallMethod({ objectNodeID: methodDetails.ObjectNodeID, methodNodeID: methodDetails.MethodNodeID, inputArguments: methodInputs })
      if (!selectedMethod || nodeSelectionKey(selectedMethod) !== selectionKey) return
      methodResult = { ...result, InputArgumentResults: result.InputArgumentResults || [], OutputArguments: result.OutputArguments || [] }
      if (result.StatusCode.includes('Good')) addToast('info', `Method call completed: ${result.StatusCode}`)
      else addToast('error', `Method call returned ${result.StatusCode}`)
    } catch (error) {
      if (!selectedMethod || nodeSelectionKey(selectedMethod) !== selectionKey) return
      methodCallError = `Method call failed: ${String(error)}`
      addToast('error', methodCallError)
    } finally {
      if (selectedMethod && nodeSelectionKey(selectedMethod) === selectionKey) methodSubmitting = false
    }
  }

  function methodObjectName() {
    if (!selectedMethod?.ParentNodeID) return '—'
    return tree.find(item => item.node.NodeID === selectedMethod?.ParentNodeID)?.node.DisplayName || selectedMethod.ParentNodeID
  }

  function openWriteConfirmation() {
    if (!inspection || !canOpenWriteConfirmation) return
    writeError = ''
    writeResult = null
    writeConfirmationSnapshot = {
      nodeID: inspection.node.NodeID,
      updateCount: inspection.updateCount,
      value: inspection.value?.Value || '',
      status: inspection.value?.Status || '',
      sourceTimestamp: inspection.value?.SourceTimestamp || '',
      serverTimestamp: inspection.value?.ServerTimestamp || ''
    }
    writeConfirmOpen = true
    if (readmeScreenshotState?.confirmationInspectionUpdate) {
      queueMicrotask(() => {
        inspection = readmeScreenshotState.confirmationInspectionUpdate as Inspection
      })
    }
  }

  function closeWriteConfirmation() {
    writeConfirmOpen = false
    writeConfirmationSnapshot = null
  }

  async function confirmVariableNodeWrite() {
    if (!inspection || !writeConfirmationSnapshot || writeConfirmationInvalidated || writeSubmitting) return
    writeSubmitting = true
    writeError = ''
    writeResult = null
    try {
      const result = await WriteVariableNodeValue({ nodeID: inspection.node.NodeID, targetValue: writeTargetValue })
      writeResult = result
      writeConfirmOpen = false
      writeConfirmationSnapshot = null
      writeTargetValue = ''
      if (result.status === 'warning') {
        addToast('info', `Variable Node Write accepted with warning: ${result.warning}`)
      } else {
        addToast('info', 'Variable Node Write accepted')
      }
    } catch (error) {
      writeError = String(error)
      addToast('error', `Variable Node Write failed: ${String(error)}`)
    } finally {
      writeSubmitting = false
    }
  }

  function variableNodeWriteDisabledReasons(current: Inspection | null, isConnected: boolean, isReadOnly: boolean, target: string) {
    const reasons: string[] = []
    if (!isConnected) reasons.push('Connect to an OPC UA Server before writing.')
    if (isReadOnly) reasons.push('Read-Only Mode is active.')
    if (!current) return [...reasons, 'Select a Variable Node in Variable Node Inspection.']
    if (current.node.NodeClass !== 'Variable') reasons.push('Selected node is not a Variable Node.')
    if (current.loadingDetails) {
      reasons.push('Write availability cannot yet be determined while Variable Node metadata is loading.')
    } else if (current.detailsError) {
      reasons.push('Write availability could not be determined because Variable Node metadata failed to load.')
      reasons.push(`Details failed to load: ${current.detailsError}`)
    } else if (!current.details?.NodeID) {
      reasons.push('Write availability cannot yet be determined because Variable Node metadata is unavailable.')
    } else {
      if (current.details.ValueRank && current.details.ValueRank !== 'Scalar') reasons.push(`Only scalar Variable Node Writes are supported; ValueRank is ${current.details.ValueRank}.`)
      if (!current.details.Writable) reasons.push(current.details.WriteAvailability || 'Effective metadata says this Variable Node is not writable in this session.')
      if (current.details.DataType && !isSupportedWriteDataType(current.details.DataType)) reasons.push(`Data type ${current.details.DataType} is not supported for Variable Node Write.`)
      if (!current.details.DataType) reasons.push('Data type is unavailable.')
    }
    if (current.stale) reasons.push('Current Live Value is stale.')
    else if (current.error || current.updateCount === 0) reasons.push('Current Live Value is unavailable.')
    const parseError = parseWriteTargetError(current.details?.DataType || '', target)
    if (parseError) reasons.push(parseError)
    return reasons
  }

  function isSupportedWriteDataType(dataType: string) {
    return ['Boolean', 'SByte', 'Int16', 'Int32', 'Int64', 'Byte', 'UInt16', 'UInt32', 'UInt64', 'Float', 'Double', 'String'].includes(dataType.trim())
  }

  function parseWriteTargetError(dataType: string, target: string) {
    return parseScalarInputError(dataType, target, 'Target Value')
  }

  function parseScalarInputError(dataType: string, target: string, label: string) {
    const trimmed = target.trim()
    if (!dataType || !isSupportedWriteDataType(dataType)) return ''
    if (dataType === 'String') return ''
    if (!trimmed) return label === 'Target Value' ? 'Enter a Target Value.' : `Enter a value for ${label}.`
    if (dataType === 'Boolean') return ['true', 'false', '1', '0', 'on', 'off', 'yes', 'no'].includes(trimmed.toLowerCase()) ? '' : `${label} must parse as Boolean.`
    if (['Float', 'Double'].includes(dataType)) {
      const decimalFloatPattern = /^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/
      const parsed = Number(trimmed)
      const inRange = Number.isFinite(parsed) && (dataType !== 'Float' || Number.isFinite(Math.fround(parsed)))
      return decimalFloatPattern.test(trimmed) && inRange ? '' : `${label} must parse as ${dataType}.`
    }
    if (!/^[+]?\d+$/.test(trimmed) && ['Byte', 'UInt16', 'UInt32', 'UInt64'].includes(dataType)) return `${label} must be an unsigned plain decimal integer for ${dataType}.`
    if (!/^[+-]?\d+$/.test(trimmed)) return `${label} must be a plain decimal integer for ${dataType}.`
    const value = BigInt(trimmed)
    const ranges: Record<string, [bigint, bigint]> = {
      SByte: [BigInt('-128'), BigInt('127')], Int16: [BigInt('-32768'), BigInt('32767')], Int32: [BigInt('-2147483648'), BigInt('2147483647')], Int64: [BigInt('-9223372036854775808'), BigInt('9223372036854775807')],
      Byte: [BigInt('0'), BigInt('255')], UInt16: [BigInt('0'), BigInt('65535')], UInt32: [BigInt('0'), BigInt('4294967295')], UInt64: [BigInt('0'), BigInt('18446744073709551615')]
    }
    const [min, max] = ranges[dataType]
    return value < min || value > max ? `${label} is outside ${dataType} range.` : ''
  }

  function writeTargetRangeWarning(current: Inspection | null, target: string) {
    if (!current?.details?.EURange || !target.trim()) return ''
    const numeric = Number(target)
    if (!Number.isFinite(numeric)) return ''
    if (numeric < current.details.EURange.Low) return `Target Value is below EURange (${current.details.EURange.Low}–${current.details.EURange.High}); this warns but does not block.`
    if (numeric > current.details.EURange.High) return `Target Value is above EURange (${current.details.EURange.Low}–${current.details.EURange.High}); this warns but does not block.`
    return ''
  }

  function isWriteConfirmationInvalidated(current: Inspection | null, snapshot: WriteConfirmationSnapshot | null) {
    if (!current || !snapshot) return false
    return current.node.NodeID !== snapshot.nodeID || current.updateCount !== snapshot.updateCount || current.value?.Value !== snapshot.value || current.value?.Status !== snapshot.status || current.value?.SourceTimestamp !== snapshot.sourceTimestamp || current.value?.ServerTimestamp !== snapshot.serverTimestamp
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
    if (nodeClass === 'Method') return 'play_circle'
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
          <span class="rounded border border-outline-variant px-sm py-xs text-xs font-bold {readOnlyMode ? 'bg-primary-container text-background' : 'bg-tertiary-container text-background'}">{readOnlyMode ? 'Read-Only Mode' : 'Changes Allowed'}</span>
          {#if readOnlyMode}
            <button class="btn-secondary" on:click={() => setReadOnlyMode(false)}>Allow changes this session</button>
          {:else}
            <button class="btn-secondary" on:click={() => setReadOnlyMode(true)}>Read-Only Mode</button>
          {/if}
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
        <section class="grid h-full min-h-[600px] gap-lg xl:grid-cols-[minmax(240px,0.8fr)_minmax(280px,0.95fr)_minmax(360px,1.25fr)]">
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
                    {#if item.node.NodeClass === 'Method'}
                      <span class="material-symbols-outlined flex h-6 w-6 items-center justify-center text-[18px] text-primary" title="Method Node">play_circle</span>
                    {:else}
                      <button class="flex h-6 w-6 items-center justify-center rounded hover:bg-surface-container-highest" on:click={() => toggleNode(item)} title="Expand or collapse">
                        {#if item.loading}<span class="text-xs text-primary">…</span>{:else}<span class="material-symbols-outlined text-[18px]">{item.expanded ? 'expand_more' : 'chevron_right'}</span>{/if}
                      </button>
                    {/if}
                    <button aria-label={`${item.node.DisplayName} ${item.node.NodeClass}`} class="min-w-0 flex-1 truncate rounded px-xs py-xs text-left {selectedNodeID === nodeSelectionKey(item.node) ? 'bg-secondary-container text-on-secondary-container' : 'text-on-surface'}" on:click={() => activateNode(item)}>
                      <span class="mr-sm font-medium">{item.node.DisplayName}</span><span class="font-mono text-xs {item.node.NodeClass === 'Method' ? 'text-primary' : 'text-on-surface-variant'}">{item.node.NodeClass}</span>
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
                  {#each searchView.results as result (nodeSelectionKey(result.node))}
                    <div role="button" tabindex="0" class="group relative block w-full cursor-pointer overflow-hidden rounded-lg border border-outline-variant bg-surface-container p-md text-left transition-colors hover:border-primary/70 {selectedNodeID === nodeSelectionKey(result.node) ? 'border-primary bg-secondary-container/40' : ''}" on:click={() => activateSearchResult(result)} on:keydown={(event) => event.key === 'Enter' && activateSearchResult(result)}>
                      <div class="absolute left-0 top-0 bottom-0 w-[2px] bg-primary {selectedNodeID === nodeSelectionKey(result.node) ? 'scale-y-100' : 'scale-y-0 group-hover:scale-y-100'} origin-top transition-transform"></div>
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
              <div class="min-w-0"><p class="label">{selectedMethod ? 'Method Call' : 'Variable Node Inspection'}</p><h2 class="truncate text-xl font-semibold">{selectedMethod?.DisplayName ?? inspection?.node?.DisplayName ?? 'No node selected'}</h2></div>
              {#if inspection}
                <div class="flex shrink-0 items-center gap-sm">
                  <button class="btn-secondary" on:click={refreshInspectionValue} disabled={refreshingNodeID === inspection.node.NodeID}>{refreshingNodeID === inspection.node.NodeID ? 'Refreshing…' : 'Refresh current value'}</button>
                  {#if inspection.watched}
                    <button class="btn-secondary" on:click={() => removeFromWatchlist(inspection?.node.NodeID || '')}>Remove from Watchlist</button>
                  {:else}
                    <button class="btn-primary" on:click={addSelectedToWatchlist}>Add to Watchlist</button>
                  {/if}
                </div>
              {/if}
            </div>
            <div class="min-h-0 flex-1 overflow-auto p-lg">
              {#if selectedMethod}
                {#if methodDetailsLoading}
                  <div class="rounded border border-outline-variant bg-surface-container-low p-md text-on-surface-variant">Loading Method metadata…</div>
                {/if}
                {#if methodDetails}
                  <div class="space-y-lg">
                    <div>
                      <div class="flex flex-wrap items-center gap-sm">
                        <span class="material-symbols-outlined text-primary">play_circle</span>
                        <span class="rounded bg-primary/10 px-sm py-xs text-sm font-semibold text-primary">Method Node</span>
                        <span class="rounded bg-surface-container-highest px-sm py-xs text-sm {methodDetails.Executable && methodDetails.UserExecutable ? 'text-emerald-400' : 'text-tertiary'}">{methodDetails.Executable && methodDetails.UserExecutable ? 'Executable for this session' : methodDetails.Executable ? 'Not executable for this session' : 'Not executable'}</span>
                      </div>
                      <p class="mt-md text-on-surface-variant">{methodDetails.Description || 'No description provided.'}</p>
                    </div>

                    <dl class="grid gap-sm text-sm sm:grid-cols-2">
                      <div class="rounded border border-outline-variant bg-surface-container-low p-sm"><dt class="label">Object Node</dt><dd class="mt-xs font-semibold">{methodObjectName()}</dd></div>
                      <div class="rounded border border-outline-variant bg-surface-container-low p-sm"><dt class="label">Method Node</dt><dd class="mt-xs font-semibold">{selectedMethod.DisplayName}</dd></div>
                      <div class="rounded border border-outline-variant bg-surface-container-low p-sm sm:col-span-2"><dt class="label">Object NodeID</dt><dd class="mt-xs break-all font-mono">{methodDetails.ObjectNodeID}</dd></div>
                      <div class="rounded border border-outline-variant bg-surface-container-low p-sm sm:col-span-2"><dt class="label">Method NodeID</dt><dd class="mt-xs break-all font-mono">{methodDetails.MethodNodeID}</dd></div>
                    </dl>

                    <section aria-labelledby="method-input-heading">
                      <div class="flex items-center justify-between"><h3 id="method-input-heading" class="text-lg font-semibold">Input arguments</h3><span class="font-mono text-xs text-on-surface-variant">{methodDetails.InputArguments.length} ordered</span></div>
                      {#if methodDetails.InputArguments.length === 0}
                        <p class="mt-sm text-sm text-on-surface-variant">This Method has no input arguments.</p>
                      {:else}
                        <div class="mt-sm space-y-sm">
                          {#each methodDetails.InputArguments as argument, index}
                            <label class="block rounded border border-outline-variant bg-surface-container-low p-md">
                              <span class="flex flex-wrap items-center justify-between gap-sm"><span class="font-semibold">{argument.Name || `Input ${index + 1}`}</span><span class="font-mono text-xs text-primary">{argument.DataType} · {argument.ValueRank}</span></span>
                              <input class="field mt-sm w-full" aria-label={argument.Name || `Input ${index + 1}`} value={methodInputs[index] || ''} disabled={!argument.Supported || argument.ValueRank !== 'Scalar' || !isSupportedWriteDataType(argument.DataType)} on:input={(event) => updateMethodInput(index, event.currentTarget.value)} on:keydown={(event) => event.key === 'Enter' && event.preventDefault()} placeholder={`Enter ${argument.DataType}`} />
                              <span class="mt-sm block text-xs text-on-surface-variant">DataType NodeID: {argument.DataTypeID || '—'} · Dimensions: {argument.ArrayDimensions?.length ? argument.ArrayDimensions.join(' × ') : 'none'}</span>
                              {#if argument.Description}<span class="mt-xs block text-sm text-on-surface-variant">{argument.Description}</span>{/if}
                            </label>
                          {/each}
                        </div>
                      {/if}
                    </section>

                    <section aria-labelledby="method-output-heading">
                      <div class="flex items-center justify-between"><h3 id="method-output-heading" class="text-lg font-semibold">Output arguments</h3><span class="font-mono text-xs text-on-surface-variant">{methodDetails.OutputArguments.length} ordered</span></div>
                      {#if methodDetails.OutputArguments.length === 0}
                        <p class="mt-sm text-sm text-on-surface-variant">This Method has no declared output arguments.</p>
                      {:else}
                        <ol class="mt-sm space-y-sm">
                          {#each methodDetails.OutputArguments as argument, index}
                            <li class="rounded border border-outline-variant bg-surface-container-low p-sm text-sm"><span class="font-semibold">{index + 1}. <span>{argument.Name || `Output ${index + 1}`}</span></span><span class="ml-sm font-mono text-xs text-primary">{argument.DataType} · {argument.ValueRank}</span><p class="mt-xs text-xs text-on-surface-variant">DataType NodeID: {argument.DataTypeID || '—'} · Dimensions: {argument.ArrayDimensions?.length ? argument.ArrayDimensions.join(' × ') : 'none'}</p>{#if argument.Description}<p class="mt-xs text-on-surface-variant">{argument.Description}</p>{/if}</li>
                          {/each}
                        </ol>
                      {/if}
                    </section>

                    {#if methodDisabledReasons.length > 0}
                      <ul class="list-disc space-y-xs pl-lg text-sm text-on-surface-variant">
                        {#each methodDisabledReasons as reason}<li>{reason}</li>{/each}
                      </ul>
                    {/if}
                    <section class="rounded border border-tertiary-container/60 bg-tertiary-container/10 p-md" aria-label="Method call review">
                      <p class="label">Review Method call</p>
                      <dl class="mt-sm space-y-xs text-sm">
                        <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Saved Connection / Endpoint</dt><dd class="text-right font-mono">{currentConnection || endpointText || 'Current session'}</dd></div>
                        <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Object Node</dt><dd class="text-right">{methodObjectName()}</dd></div>
                        <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Method Node</dt><dd class="text-right">{selectedMethod.DisplayName}</dd></div>
                        <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Object NodeID</dt><dd class="break-all text-right font-mono">{methodDetails.ObjectNodeID}</dd></div>
                        <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Method NodeID</dt><dd class="break-all text-right font-mono">{methodDetails.MethodNodeID}</dd></div>
                        {#each methodDetails.InputArguments as argument, index}<div class="flex justify-between gap-md"><dt class="text-on-surface-variant">{argument.Name || `Input ${index + 1}`}</dt><dd class="break-all text-right font-mono">{methodInputs[index] || '—'}</dd></div>{/each}
                      </dl>
                    </section>
                    <button class="btn-primary w-full" disabled={!canCallMethod} on:click={submitMethodCall}>{methodSubmitting ? 'Calling…' : 'Call Method'}</button>

                    {#if methodCallError}<div class="rounded border border-error-container bg-error-container/20 p-md text-error">{methodCallError}</div>{/if}
                    {#if methodResult}
                      <section class="rounded border {methodResult.StatusCode.includes('Good') ? 'border-primary/50 bg-primary/10' : 'border-error-container bg-error-container/20'} p-md" aria-label="Method call result">
                        <p class="label">StatusCode</p><p class="mt-xs break-all font-mono text-lg">{methodResult.StatusCode}</p>
                        {#if methodResult.InputArgumentResults.length}<p class="mt-md label">Input argument results</p><ol class="mt-xs list-decimal pl-lg font-mono text-sm">{#each methodResult.InputArgumentResults as status}<li>{status}</li>{/each}</ol>{/if}
                        <p class="mt-md label">Raw outputs</p>
                        {#if methodResult.OutputArguments.length}<ol class="mt-xs space-y-xs">{#each methodResult.OutputArguments as output, index}<li class="font-mono text-sm">{index + 1}. <span class="text-primary">{output.DataType}</span> <span>{output.Value}</span></li>{/each}</ol>{:else}<p class="mt-xs text-sm text-on-surface-variant">No output values returned.</p>{/if}
                      </section>
                    {/if}
                  </div>
                {/if}
                {#if !methodDetails}
                  <button class="btn-primary mt-md w-full" disabled>Call Method</button>
                  {#if methodDisabledReasons.length > 0}<ul class="mt-md list-disc space-y-xs pl-lg text-sm text-on-surface-variant">{#each methodDisabledReasons as reason}<li>{reason}</li>{/each}</ul>{/if}
                {/if}
              {:else if inspection}
                <div class="grid gap-md lg:grid-cols-3">
                  <div class="panel bg-surface-container-low p-md"><p class="label">Live Value</p><p class="mt-sm font-mono text-2xl text-primary">{inspection.value?.Value || '—'}</p></div>
                  <div class="panel bg-surface-container-low p-md"><p class="label">Status</p><p class="mt-sm font-mono text-sm {inspection.stale ? 'text-tertiary' : 'text-emerald-400'}">{inspection.stale ? 'Stale' : inspection.value?.Status || 'Waiting'}</p></div>
                  <div class="panel bg-surface-container-low p-md"><p class="label">Updates</p><p class="mt-sm font-mono text-2xl">{inspection.updateCount}</p></div>
                </div>
                <div class="mt-md rounded border border-outline-variant bg-surface-container-low p-md">
                  <p class="label">Effective Write Availability</p>
                  <p class="mt-xs text-lg font-semibold {inspection.details?.Writable ? 'text-primary' : 'text-on-surface'}">{inspection.details?.WriteAvailability || 'Write availability not confirmed for this user'}</p>
                </div>
                <div class="mt-md rounded border border-outline-variant bg-surface-container-low p-md">
                  <div class="flex items-start justify-between gap-md">
                    <div>
                      <p class="label">Variable Node Write</p>
                      <h3 class="mt-xs text-lg font-semibold">Write value</h3>
                      <p class="mt-xs text-sm text-on-surface-variant">Available only from Variable Node Inspection. Every write requires confirmation and cannot be submitted with Enter.</p>
                    </div>
                    <span class="rounded px-sm py-xs text-xs font-bold {readOnlyMode ? 'bg-primary-container text-background' : 'bg-tertiary-container text-background'}">{readOnlyMode ? 'Read-Only Mode' : 'Changes Allowed'}</span>
                  </div>
                  <div class="mt-md grid gap-sm lg:grid-cols-[1fr_auto]">
                    <label class="space-y-xs">
                      <span class="label">Target Value</span>
                      <input class="field w-full" bind:value={writeTargetValue} on:keydown={(event) => event.key === 'Enter' && event.preventDefault()} placeholder={inspection.details?.DataType ? `Enter ${inspection.details.DataType}` : 'Waiting for data type'} />
                    </label>
                    <div class="flex items-end">
                      <button class="btn-primary h-9" disabled={!canOpenWriteConfirmation} on:click={openWriteConfirmation}>{writeSubmitting ? 'Writing…' : 'Write value'}</button>
                    </div>
                  </div>
                  {#if writeDisabledReasons.length > 0}
                    <ul class="mt-md list-disc space-y-xs pl-lg text-sm text-on-surface-variant">
                      {#each writeDisabledReasons as reason}<li>{reason}</li>{/each}
                    </ul>
                  {/if}
                  {#if writeRangeWarning}<div class="mt-md rounded border border-tertiary-container bg-tertiary-container/10 p-sm text-sm text-tertiary">{writeRangeWarning}</div>{/if}
                  {#if writeStatusWarning}<div class="mt-md rounded border border-tertiary-container bg-tertiary-container/10 p-sm text-sm text-tertiary">{writeStatusWarning}</div>{/if}
                  {#if writeError}<div class="mt-md rounded border border-error-container bg-error-container/20 p-sm text-sm text-error">{writeError}</div>{/if}
                  {#if writeResult}
                    <div class="mt-md rounded border {writeResult.status === 'warning' ? 'border-tertiary-container bg-tertiary-container/10 text-tertiary' : 'border-primary/50 bg-primary/10 text-on-surface'} p-sm text-sm">
                      {#if writeResult.status === 'warning'}{writeResult.warning}{:else}Write accepted. Read-back: {writeResult.readBack?.Value} ({writeResult.readBack?.Status}){/if}
                    </div>
                  {/if}
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
                      <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">AccessLevel</dt><dd>{inspection.details?.AccessLevel || '—'}</dd></div>
                      <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">UserAccessLevel</dt><dd>{inspection.details?.UserAccessLevelAvailable ? inspection.details.UserAccessLevel : '—'}</dd></div>
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
                <div class="flex h-full items-center justify-center text-center text-on-surface-variant">Select a Variable Node or Method Node from the Address Space to inspect it.</div>
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

  {#if writeConfirmOpen && inspection && writeConfirmationSnapshot}
    <div class="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-lg">
      <div class="w-full max-w-2xl rounded-lg border border-outline-variant bg-surface p-lg shadow-2xl" role="dialog" aria-modal="true" aria-labelledby="write-confirmation-heading">
        <div class="flex items-start justify-between gap-md">
          <div>
            <p class="label">Confirm Variable Node Write</p>
            <h2 id="write-confirmation-heading" class="mt-xs text-2xl font-semibold">This changes the OPC UA Server</h2>
            <p class="mt-sm text-sm text-on-surface-variant">Review the current Live Value and Target Value. If the Live Value changes while this confirmation is open, confirmation is invalidated.</p>
          </div>
          <button class="rounded p-xs hover:bg-surface-container-high" on:click={closeWriteConfirmation} title="Cancel"><span class="material-symbols-outlined">close</span></button>
        </div>
        <dl class="mt-lg grid gap-sm text-sm sm:grid-cols-2">
          <div class="rounded border border-outline-variant bg-surface-container-low p-sm"><dt class="label">Saved Connection / Endpoint</dt><dd class="mt-xs break-all font-mono">{currentConnection || endpointText || 'Current session'}</dd></div>
          <div class="rounded border border-outline-variant bg-surface-container-low p-sm"><dt class="label">Variable Node</dt><dd class="mt-xs font-semibold">{inspection.node.DisplayName}</dd></div>
          <div class="rounded border border-outline-variant bg-surface-container-low p-sm sm:col-span-2"><dt class="label">NodeID</dt><dd class="mt-xs break-all font-mono">{inspection.node.NodeID}</dd></div>
          <div class="rounded border border-outline-variant bg-surface-container-low p-sm"><dt class="label">Current Live Value</dt><dd class="mt-xs font-mono text-primary">{writeConfirmationSnapshot.value || '—'}</dd></div>
          <div class="rounded border border-outline-variant bg-surface-container-low p-sm"><dt class="label">Current Status</dt><dd class="mt-xs font-mono">{writeConfirmationSnapshot.status || '—'}</dd></div>
          <div class="rounded border border-outline-variant bg-surface-container-low p-sm"><dt class="label">Target Value</dt><dd class="mt-xs font-mono text-tertiary">{writeTargetValue}</dd></div>
          <div class="rounded border border-outline-variant bg-surface-container-low p-sm"><dt class="label">Data Type</dt><dd class="mt-xs font-mono">{inspection.details?.DataType || '—'}</dd></div>
          <div class="rounded border border-outline-variant bg-surface-container-low p-sm"><dt class="label">Source Timestamp</dt><dd class="mt-xs font-mono">{compactDateTime(writeConfirmationSnapshot.sourceTimestamp)}</dd></div>
          <div class="rounded border border-outline-variant bg-surface-container-low p-sm"><dt class="label">Server Timestamp</dt><dd class="mt-xs font-mono">{compactDateTime(writeConfirmationSnapshot.serverTimestamp)}</dd></div>
        </dl>
        {#if writeConfirmationInvalidated}
          <div class="mt-md rounded border border-error-container bg-error-container/20 p-md text-error">Current Live Value changed while confirmation was open. Cancel and review the new value before writing.</div>
        {/if}
        {#if writeRangeWarning}<div class="mt-md rounded border border-tertiary-container bg-tertiary-container/10 p-md text-tertiary">{writeRangeWarning}</div>{/if}
        {#if writeStatusWarning}<div class="mt-md rounded border border-tertiary-container bg-tertiary-container/10 p-md text-tertiary">{writeStatusWarning}</div>{/if}
        <div class="mt-lg flex justify-end gap-sm">
          <button class="btn-secondary" on:click={closeWriteConfirmation}>Cancel</button>
          <button class="btn-primary" disabled={writeConfirmationInvalidated || writeSubmitting} on:click={confirmVariableNodeWrite}>{writeSubmitting ? 'Writing…' : 'Confirm write'}</button>
        </div>
      </div>
    </div>
  {/if}

  <div class="pointer-events-none fixed right-md top-md z-50 space-y-sm">
    {#each toasts as toast}
      <div class="w-96 rounded border border-outline-variant bg-surface-container-high p-md text-sm shadow-xl {toast.level === 'error' ? 'text-error' : 'text-on-surface'}">{toast.message}</div>
    {/each}
  </div>
</div>
