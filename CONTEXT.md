# OPC UA Studio

A desktop OPC UA client for automation engineers who need to inspect and interact with existing OPC UA Servers.

## Language

**OPC UA Studio**:
A desktop OPC UA client that connects to existing OPC UA Servers so automation engineers can inspect and interact with them.
_Avoid_: OPC UA Client TUI, Control Gateway, OPC UA Server UI

**Saved Connection**:
A locally stored set of non-secret details used to reconnect to an OPC UA Server, often labeled with a user-defined name such as “Control Gateway”.
_Avoid_: Profile, bookmark, credential, workspace

**Read-Only Mode**:
A state of OPC UA Studio where it does not perform write operations or mutating method calls against the OPC UA Server. Non-mutating query methods may still be used to inspect server information.
_Avoid_: Safe mode, view-only mode, no-write mode

**Client Certificate**:
A certificate presented by OPC UA Studio when connecting to an OPC UA Server endpoint that requires signed or encrypted communication.
_Avoid_: User certificate, server certificate, credential file

**Automation Engineer**:
A practitioner who configures, commissions, troubleshoots, or maintains industrial automation systems.
_Avoid_: Developer, operator, user

**OPC UA Server**:
An industrial automation endpoint that exposes data, metadata, events, and operations through the OPC UA protocol.
_Avoid_: Server implementation, backend

**Address Space**:
The browsable structure of an OPC UA Server, containing nodes and their relationships.
_Avoid_: Tag tree, menu, file tree

**Search Result**:
An item returned by searching an OPC UA Server's functional names or Address Space metadata, pointing to a specific node or semantic reference in that server.
_Avoid_: Asset, tag result, search hit

**Alias Name**:
A human-readable functional name exposed by an OPC UA Server that refers to one or more nodes independently of where those nodes live in the Address Space.
_Avoid_: Asset name, friendly tag, display alias

**Address Space Search**:
Search across Alias Names and Address Space metadata to find Search Results without requiring the Automation Engineer to navigate the tree first.
_Avoid_: Asset search, tag search, global search

**Rate-Limited Browsing**:
Browsing or indexing the Address Space at a bounded request rate to reduce load on an OPC UA Server.
_Avoid_: Gentle browsing, crawl, aggressive indexing

**Shallow Address Space Indexing**:
Bounded discovery of nearby Address Space metadata that expands Address Space Search without promising exhaustive coverage of an OPC UA Server.
_Avoid_: Full crawl, deep indexing, complete server scan

**Object Node**:
A node in the Address Space that represents an entity, grouping, or component and may organize related Variable Nodes.
_Avoid_: Asset, device card, folder

**Variable Node**:
A node in the Address Space that represents a readable, and sometimes writable, process value or state.
_Avoid_: Tag, point, field

**Observed Variable Node**:
A Variable Node whose Live Value is watched during the current Troubleshooting Session and remains available in Session Trend.
_Avoid_: Inspected node, trended node, sampled node, monitored node

**Live Value**:
The current value of a Variable Node together with its health and timestamp information.
_Avoid_: Reading, datapoint, sample

**Stale Value**:
A previously observed Live Value that may no longer represent the current state of the OPC UA Server.
_Avoid_: Cached value, invalid value, old reading

**Out-of-Range**:
A condition where a numeric Live Value falls outside range metadata exposed by the OPC UA Server.
_Avoid_: Alarm, alert, warning

**Troubleshooting Session**:
A focused investigation of a live OPC UA Server to understand current values, metadata, status, and connectivity problems.
_Avoid_: Monitoring session, dashboard, commissioning workflow

**Variable Node Inspection**:
The focused view of one Variable Node during a Troubleshooting Session, combining its Live Value, metadata, health, stale state, and out-of-range status.
_Avoid_: selected value panel, node details workflow

**Watchlist**:
A user-selected set of Variable Nodes whose Live Values remain readily available during a Troubleshooting Session.
_Avoid_: Dashboard, monitor, pinned nodes, Live Monitor, Live View

**Session Trend**:
A temporary view of Live Value updates observed for a Variable Node during the current Troubleshooting Session.
_Avoid_: Historian, chart history, Trend Dashboard
