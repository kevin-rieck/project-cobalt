type Tab = 'connections' | 'address-space' | 'watchlist' | 'session-trend' | 'logs'

type AddressNode = {
  ParentNodeID?: string
  NodeID: string
  DisplayName: string
  BrowseName: string
  NodeClass: string
}

const now = '2026-06-25T14:32:18Z'
const later = '2026-06-25T14:32:21Z'

const nodes = {
  objects: { NodeID: 'i=85', DisplayName: 'Objects', BrowseName: 'Objects', NodeClass: 'Object' },
  line1: { NodeID: 'ns=2;s=Plant.Line1', DisplayName: 'Packaging Line 1', BrowseName: '2:PackagingLine1', NodeClass: 'Object' },
  filler: { NodeID: 'ns=2;s=Plant.Line1.Filler', DisplayName: 'Filler Station', BrowseName: '2:FillerStation', NodeClass: 'Object' },
  conveyor: { NodeID: 'ns=2;s=Plant.Line1.Conveyor', DisplayName: 'Infeed Conveyor', BrowseName: '2:InfeedConveyor', NodeClass: 'Object' },
  temp: { NodeID: 'ns=2;s=Plant.Line1.Filler.Temperature', DisplayName: 'Filler Temperature', BrowseName: '2:FillerTemperature', NodeClass: 'Variable' },
  pressure: { NodeID: 'ns=2;s=Plant.Line1.Filler.Pressure', DisplayName: 'Bowl Pressure', BrowseName: '2:BowlPressure', NodeClass: 'Variable' },
  speed: { NodeID: 'ns=2;s=Plant.Line1.Conveyor.Speed', DisplayName: 'Conveyor Speed', BrowseName: '2:ConveyorSpeed', NodeClass: 'Variable' },
  rejectCount: { NodeID: 'ns=2;s=Plant.Line1.RejectCount', DisplayName: 'Reject Count', BrowseName: '2:RejectCount', NodeClass: 'Variable' },
  methods: { ParentNodeID: 'i=85', NodeID: 'ns=3;s=Demo.CTT.Methods', DisplayName: 'Methods', BrowseName: '3:Methods', NodeClass: 'Object' },
  methodIO: { ParentNodeID: 'ns=3;s=Demo.CTT.Methods', NodeID: 'ns=3;s=Demo.CTT.Methods.MethodIO', DisplayName: 'MethodIO', BrowseName: '3:MethodIO', NodeClass: 'Method' }
} satisfies Record<string, AddressNode>

const tree = [
  { key: 'root:i=85', node: nodes.objects, depth: 0, expanded: true, childrenLoaded: true, loading: false, error: '' },
  { key: nodes.line1.NodeID, node: nodes.line1, depth: 1, expanded: true, childrenLoaded: true, loading: false, error: '' },
  { key: nodes.filler.NodeID, node: nodes.filler, depth: 2, expanded: true, childrenLoaded: true, loading: false, error: '' },
  { key: nodes.temp.NodeID, node: nodes.temp, depth: 3, expanded: false, childrenLoaded: false, loading: false, error: '' },
  { key: nodes.pressure.NodeID, node: nodes.pressure, depth: 3, expanded: false, childrenLoaded: false, loading: false, error: '' },
  { key: nodes.conveyor.NodeID, node: nodes.conveyor, depth: 2, expanded: true, childrenLoaded: true, loading: false, error: '' },
  { key: nodes.speed.NodeID, node: nodes.speed, depth: 3, expanded: false, childrenLoaded: false, loading: false, error: '' },
  { key: nodes.rejectCount.NodeID, node: nodes.rejectCount, depth: 2, expanded: false, childrenLoaded: false, loading: false, error: '' },
  { key: nodes.methods.NodeID, node: nodes.methods, depth: 1, expanded: true, childrenLoaded: true, loading: false, error: '' },
  { key: nodes.methodIO.NodeID, node: nodes.methodIO, depth: 2, expanded: false, childrenLoaded: true, loading: false, error: '' }
]

const inspection = {
  node: nodes.temp,
  value: { Value: '83.7', Status: 'Good', SourceTimestamp: now, ServerTimestamp: now },
  details: {
    NodeID: nodes.temp.NodeID,
    Description: 'Temperature at filler bowl outlet',
    DataType: 'Double',
    AccessLevel: 'CurrentRead | CurrentWrite',
    UserAccessLevel: 'CurrentRead | CurrentWrite',
    UserAccessLevelAvailable: true,
    Writable: true,
    WriteAvailability: 'Writable in this session',
    ValueRank: 'Scalar',
    ArrayDimensions: '',
    EngineeringUnit: '°C',
    EURange: { Low: 0, High: 100 },
    InstrumentRange: { Low: -10, High: 120 }
  },
  subscribing: true,
  loadingDetails: false,
  stale: false,
  outOfRange: '',
  updateCount: 184,
  watched: true,
  error: '',
  detailsError: ''
}

const changedInspection = {
  ...inspection,
  value: { ...inspection.value, Value: '84.0', SourceTimestamp: later, ServerTimestamp: later },
  updateCount: inspection.updateCount + 1
}

function inspectionForRequest(requested: string): typeof inspection {
  switch (requested) {
    case 'write-metadata-loading':
      return {
        ...inspection,
        details: { ...inspection.details, NodeID: '', DataType: '' },
        loadingDetails: true
      }
    case 'write-metadata-failed':
      return {
        ...inspection,
        details: { ...inspection.details, NodeID: '', DataType: '' },
        detailsError: 'BadAttributeIdInvalid'
      }
    case 'write-metadata-unavailable':
      return {
        ...inspection,
        details: { ...inspection.details, NodeID: '', DataType: '' }
      }
    case 'write-live-value-stale':
      return { ...inspection, stale: true }
    case 'write-live-value-unavailable':
      return {
        ...inspection,
        value: { Value: '', Status: '', SourceTimestamp: '', ServerTimestamp: '' },
        updateCount: 0
      }
    case 'write-status-warning':
      return { ...inspection, value: { ...inspection.value, Status: 'UncertainLastUsableValue' } }
    case 'write-data-type-unsupported':
      return { ...inspection, details: { ...inspection.details, DataType: 'DateTime' } }
    case 'write-value-rank-non-scalar':
      return { ...inspection, details: { ...inspection.details, ValueRank: 'OneDimension' } }
    case 'write-metadata-non-writable':
      return {
        ...inspection,
        details: {
          ...inspection.details,
          UserAccessLevel: 'CurrentRead',
          Writable: false,
          WriteAvailability: 'Read-only in this session'
        }
      }
    default:
      return inspection
  }
}

const watchlist = [
  { node: nodes.temp, value: { Value: '83.7', Status: 'Good', SourceTimestamp: now, ServerTimestamp: now }, dataType: 'Double', engineeringUnit: '°C', stale: false, outOfRange: '', updateCount: 184, error: '', detailsError: '' },
  { node: nodes.pressure, value: { Value: '6.8', Status: 'Good', SourceTimestamp: now, ServerTimestamp: now }, dataType: 'Float', engineeringUnit: 'bar', stale: false, outOfRange: '', updateCount: 177, error: '', detailsError: '' },
  { node: nodes.speed, value: { Value: '1.42', Status: 'UncertainLastUsableValue', SourceTimestamp: now, ServerTimestamp: now }, dataType: 'Double', engineeringUnit: 'm/s', stale: false, outOfRange: '', updateCount: 96, error: '', detailsError: '' },
  { node: nodes.rejectCount, value: { Value: '128', Status: 'Good', SourceTimestamp: later, ServerTimestamp: later }, dataType: 'UInt32', engineeringUnit: '', stale: false, outOfRange: 'Above expected range 0–100', updateCount: 23, error: '', detailsError: '' }
]

const searchResults = [nodes.temp, nodes.pressure, nodes.speed].map((node, index) => ({
  node,
  matchKind: index === 0 ? 'DisplayName' : 'BrowseName',
  matchText: index === 0 ? node.DisplayName : node.BrowseName,
  source: 'Browsed Address Space metadata',
  score: 100 - index * 7
}))

const methodSearchResults = [{
  node: nodes.methodIO,
  matchKind: 'DisplayName',
  matchText: 'MethodIO',
  source: 'Browsed Address Space metadata',
  score: 100
}]

const sessionTrend = {
  nodes: [
    { node: nodes.temp, latestValue: '83.7 °C', status: 'Good', pointCount: 12 },
    { node: nodes.pressure, latestValue: '6.8 bar', status: 'Good', pointCount: 12 },
    { node: nodes.speed, latestValue: '1.42 m/s', status: 'UncertainLastUsableValue', pointCount: 8 }
  ],
  points: Array.from({ length: 12 }, (_, index) => ({
    value: `${(82.8 + index * 0.08).toFixed(1)} °C`,
    status: 'Good',
    timestamp: `2026-06-25T14:${String(21 + index).padStart(2, '0')}:18Z`,
    sourceTimestamp: `2026-06-25T14:${String(21 + index).padStart(2, '0')}:18Z`,
    serverTimestamp: `2026-06-25T14:${String(21 + index).padStart(2, '0')}:18Z`,
    receivedAt: `2026-06-25T14:${String(21 + index).padStart(2, '0')}:19Z`
  }))
}

export type ReadmeScreenshotState = {
  activeTab: Tab
  connected: boolean
  readOnlyMode: boolean
  currentConnection: string
  tree: typeof tree
  selectedNodeID: string
  selectedMethod: AddressNode | null
  inspection: typeof inspection | null
  confirmationInspectionUpdate: typeof inspection | null
  logs: Array<{ timestamp: string; level: string; message: string }>
  receiveRuntimeEvents: boolean
  watchlist: typeof watchlist
  sessionTrend: typeof sessionTrend
  focusedTrendNodeID: string
  searchQuery: string
  searchView: { query: string; results: typeof searchResults; status: string }
  savedConnections: Array<Record<string, string>>
  endpoints: Array<Record<string, string | number | string[]>>
}

export function getReadmeScreenshotState(): ReadmeScreenshotState | null {
  if (typeof window === 'undefined') return null
  const params = new URLSearchParams(window.location.search)
  const requested = params.get('screenshot')
  if (!requested) return null

  const tabByRequest: Record<string, Tab> = {
    hero: 'address-space',
    search: 'address-space',
    inspection: 'address-space',
    watchlist: 'watchlist',
    trend: 'session-trend',
    connections: 'connections',
    'connections-empty': 'connections'
  }

  const savedConnections = requested === 'connections-empty'
    ? []
    : [
      { id: 'control-gateway', name: 'Control Gateway', endpoint: 'opc.tcp://192.168.10.42:4840', securityPolicy: 'Basic256Sha256', securityMode: 'SignAndEncrypt', authType: 'Anonymous', serverCertificateThumbprint: '8A:91:4F:2C:67:12:EB:44', createdAt: now, updatedAt: now, lastConnectedAt: now }
    ]

  return {
    activeTab: tabByRequest[requested] || 'address-space',
    connected: true,
    readOnlyMode: requested === 'method-call-read-only' || (!requested.startsWith('write-') && !requested.startsWith('method-call')) || requested === 'write-read-only',
    currentConnection: 'Control Gateway',
    tree,
    selectedNodeID: requested.startsWith('method-call') ? nodes.methodIO.NodeID : nodes.temp.NodeID,
    selectedMethod: requested.startsWith('method-call') ? nodes.methodIO : null,
    inspection: requested.startsWith('method-call') ? null : inspectionForRequest(requested),
    confirmationInspectionUpdate: requested === 'write-confirmation-live-value-change' ? changedInspection : null,
    logs: [],
    receiveRuntimeEvents: requested === 'write-feedback-events' || requested === 'method-call-events',
    watchlist,
    sessionTrend,
    focusedTrendNodeID: nodes.temp.NodeID,
    searchQuery: requested === 'connections' ? '' : requested.startsWith('method-call') ? 'method' : 'filler',
    searchView: requested.startsWith('method-call')
      ? { query: 'method', results: methodSearchResults as typeof searchResults, status: '1 Search Result found in browsed Address Space metadata.' }
      : { query: 'filler', results: searchResults, status: '3 Search Results found in browsed Address Space metadata.' },
    savedConnections,
    endpoints: [
      { URL: 'opc.tcp://192.168.10.42:4840', SecurityPolicy: 'Basic256Sha256', SecurityMode: 'SignAndEncrypt', SecurityLevel: 3, UserTokenTypes: ['Anonymous', 'UserName'], ServerThumbprint: '8A:91:4F:2C:67:12:EB:44' },
      { URL: 'opc.tcp://192.168.10.42:4840', SecurityPolicy: 'None', SecurityMode: 'None', SecurityLevel: 0, UserTokenTypes: ['Anonymous'], ServerThumbprint: '' }
    ]
  }
}
