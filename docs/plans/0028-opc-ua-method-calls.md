# OPC UA Method calls implementation plan

Issue: [#28](https://github.com/kevin-rieck/project-cobalt/issues/28)

## Outcome and scope

OPC UA Studio v1 will expose Method Nodes already discovered while browsing or searching an OPC UA Server's Address Space. Selecting a Method Node opens a Method Call panel where an Automation Engineer can inspect the Method, enter supported input arguments, deliberately call it, and see the returned `StatusCode` and raw output arguments.

The plan does not implement Method calls. Saved call presets, exhaustive DataType support, arrays, and additional safety policy are deferred.

## Decisions

### Discovery and ownership

`BrowseChildren` already requests all node classes through `HierarchicalReferences`, so Method Nodes are returned today as `AddressNode{NodeClass: "Method"}`. Shallow Address Space Indexing also records those results, although it only recurses through Object and View Nodes.

A Method call requires both `ObjectId` and `MethodId`. Add `ParentNodeID` to `opcua.AddressNode` and populate it from the ID passed to `BrowseChildren`. This keeps Method ownership with browse/search results instead of making the frontend reconstruct it. A Method Node reached from the Address Space tree or Address Space Search can therefore open the same flow.

The same Method Node ID can be referenced by multiple Object Nodes (the target server does this for standard condition Methods). Address Space Search currently deduplicates only by Node ID, which would retain an arbitrary owner and could call the Method on the wrong Object Node. Change its internal identity for Method Nodes to the `(ParentNodeID, NodeID)` pair and allow one Search Result per owning Object Node. Keep Node ID identity for other node classes unless a broader reference-identity change is independently justified.

V1 does not add an Object Node context menu that performs another browse. Selecting or activating a visible Method Node is the action that opens the Method Call panel. This fits the current tree and search interaction model without introducing a second discovery path.

### Method metadata

Add these app-level protocol models under `internal/opcua`:

```go
type MethodArgument struct {
    Name, DataType, DataTypeID, ValueRank, Description string
    ArrayDimensions []uint32
    Supported bool
}

type MethodDetails struct {
    ObjectNodeID, MethodNodeID, Description string
    Executable, UserExecutable bool
    InputArguments, OutputArguments []MethodArgument
}

type MethodCallResult struct {
    StatusCode string
    InputArgumentResults []string
    OutputArguments []MethodArgumentValue
}

type MethodArgumentValue struct {
    DataType string
    Value string
}
```

`Client` gains `ReadMethodDetails(ctx, objectNodeID, methodNodeID)` and `CallMethod(ctx, objectNodeID, methodNodeID, inputs []ScalarValue)`. The concrete client reads Method `Description`, `Executable`, and `UserExecutable`, browses `HasProperty` for `InputArguments` and `OutputArguments`, and decodes each `*ua.ExtensionObject` to `*ua.Argument` (name, DataType NodeID, ValueRank, dimensions, and description). Missing argument properties mean an empty list. A malformed property is an inspection error rather than an empty signature.

Before calling, parse both NodeIDs, build `ua.CallMethodRequest`, invoke the existing `gopcua.Client.Call`, and project the service result. A non-Good Method `StatusCode` remains a returned result so the UI can display it; transport/session failures remain Go errors. Raw outputs are stable strings produced at the OPC UA boundary with their Variant DataType names, rather than exposing `ua.Variant` through Wails.

### Wails boundary and validation

Expose two methods on `App`:

```go
type MethodNodeRequest struct { ObjectNodeID, MethodNodeID string }
type MethodCallRequest struct {
    ObjectNodeID, MethodNodeID string
    InputArguments []string
}

func (a *App) GetMethodDetails(MethodNodeRequest) (opcua.MethodDetails, error)
func (a *App) CallMethod(MethodCallRequest) (opcua.MethodCallResult, error)
```

`GetMethodDetails` requires a connected session but is allowed in Read-Only Mode. `CallMethod` must, at the backend boundary:

1. require a connected session;
2. reject while Read-Only Mode is active;
3. re-read Method metadata immediately before execution;
4. require both `Executable` and `UserExecutable`;
5. require exactly the declared number of input arguments;
6. reject every unsupported DataType or non-scalar ValueRank;
7. parse all values before sending any request; and
8. log the Object Node ID, Method Node ID, returned StatusCode, and failures, but never log argument values.

Re-reading metadata makes the backend authoritative if the UI is stale or bypassed. Do not retain Method call state in `App`; the panel owns transient inputs and the latest result.

### Read-Only Mode

Method discovery and inspection are read operations and remain available in Read-Only Mode. V1 blocks **every** Method execution in Read-Only Mode because OPC UA Method metadata does not reliably identify whether a call mutates the OPC UA Server. Non-mutating exceptions can be designed later if the app gains an explicit trust/classification mechanism.

The existing session control must be broadened from write-specific language:

- confirmation: “Allow changes to the OPC UA Server?”;
- enabled state: “Changes Allowed” (Read-Only Mode remains the disabled state);
- explanation: Variable Node Writes and Method calls are enabled until disconnect or Read-Only Mode is restored.

Disconnect still restores Read-Only Mode. The Method Call panel remains inspectable but disables `Call Method` with “Read-Only Mode is active.” The panel itself provides the deliberate review step: it shows Saved Connection/endpoint, Object Node, Method Node, NodeIDs, and entered arguments immediately above the call button. No additional confirmation modal is required for v1.

### Input argument support

V1 supports scalar (`ValueRank == -1`) built-in values already handled by the repository's proven scalar parser:

- `Boolean`;
- `SByte`, `Int16`, `Int32`, `Int64`;
- `Byte`, `UInt16`, `UInt32`, `UInt64`;
- `Float`, `Double`; and
- `String`.

Extract the current parser from write-specific naming into a shared scalar argument/value codec, preserving strict decimal/range behavior. Variable Node Write and Method calls both consume it. Metadata still displays unsupported arguments, but the corresponding input is disabled and explains the unsupported DataType or ValueRank; the call button remains disabled.

Defer arrays/matrices, `ByteString`, `DateTime`, `Guid`, `NodeId`, `QualifiedName`, `LocalizedText`, enums, structures/ExtensionObjects, custom namespace DataTypes, optional arguments, and null values. Outputs of any decodable Variant type are display-only raw strings and do not imply matching input support.

### Frontend flow

1. Render Method Nodes with a distinct method icon/label in the existing Address Space tree and Address Space Search results.
2. Selecting/activating one calls `GetMethodDetails({objectNodeID: ParentNodeID, methodNodeID: NodeID})`, clears Variable Node Inspection, and opens the right-side Method Call panel.
3. Show Method display name, description, Object Node ID, Method Node ID, executable state, and ordered input/output argument metadata.
4. Render one labelled text input per supported input argument. Show DataType, ValueRank, dimensions, and description alongside each argument. Unsupported arguments remain visible with a reason.
5. Validate with the same rules as the backend. Disable the button while disconnected, Read-Only, metadata is loading/failed, not executable for the session, unsupported, invalid, or submitting.
6. Keep the review summary and `Call Method` button in the panel. Enter does not submit.
7. On success or a non-Good call result, show StatusCode and ordered raw output values inline. Use a toast for a concise success/failure notification. Clear the previous result whenever the selected Method or any input changes.
8. On disconnect, clear the Method panel and transient values.

## Target-server validation (`localhost:48010`)

Validated on 2026-07-18 against the running Unified Automation C++ demo server using `github.com/gopcua/opcua v0.8.0`:

- The endpoint advertises None, Basic256Sha256, Aes128_Sha256_RsaOaep, and Aes256_Sha256_RsaPss policies. Anonymous/None permits the v1 validation path.
- A bounded browse from Objects found Method Nodes, and `BrowseChildren`'s existing all-node-class mask is sufficient.
- Stable validation object: `ns=3;s=Demo.CTT.Methods`.
- Stable Method: `ns=3;s=Demo.CTT.Methods.MethodIO` (“Adds 2 unsigned integers”). It is executable and user-executable for Anonymous/None.
- `InputArguments`: `Summand1` and `Summand2`, both scalar `UInt32` (`i=7`).
- `OutputArguments`: `Sum`, scalar `UInt32` (`i=7`).
- Calling it with `20` and `22` returned `StatusGood`, Good per-input results, and raw output `uint32(42)`.
- The same object also exposes input-only, output-only, and no-argument Methods, useful for metadata edge cases.
- Some secure-only nodes return `BadSecurityModeInsufficient` when their argument properties are read over None. Metadata failures must therefore be surfaced honestly rather than interpreted as zero arguments.

The target confirms scalar `UInt32` as the required tracer path. The broader scalar set is low-risk because it reuses the existing typed codec, but each type still needs unit coverage independent of this server.

## Automated test strategy

Tests stay at public seams.

### `internal/opcua.Client`

- Unit-test argument ExtensionObject projection, unsupported rank/DataType classification, raw Variant projection, malformed argument metadata, and shared scalar parsing.
- Cover two Object Nodes that reference the same Method Node ID so search and execution retain the selected owner.
- Extend the environment-gated integration suite with `TERMUA_METHOD_TEST_ENDPOINT`. Against `localhost:48010`, browse the Methods object, inspect MethodIO, assert its signature, call `20 + 22`, and assert `StatusGood` plus `42`.
- Keep the integration test opt-in so normal CI does not depend on a workstation service.

### `App` Wails boundary

Using the existing recording-client seam in `app_test.go`, cover details in Read-Only Mode; disconnected calls; Read-Only rejection; executable/user-executable rejection; fresh metadata revalidation; argument count/type/rank rejection; typed argument forwarding; Good and non-Good result propagation; redacted diagnostic logging; and reset behavior after disconnect.

### Frontend Wails seam

Add Playwright scenarios with stubbed `window.go.main.App`: Method selection and metadata display, input validation, unsupported metadata reasons, Read-Only gating, executable gating, no Enter submission, in-flight disabling, Good result/raw outputs, non-Good StatusCode, transport error, selection/result reset, and disconnect cleanup. Run `svelte-check` and the Vite production build as usual.

### Completion commands

Run focused Go and Playwright tests during each slice, then once at the end:

```sh
go test ./...
npm --prefix frontend run check
npm --prefix frontend run test:ui
npm --prefix frontend run build
TERMUA_METHOD_TEST_ENDPOINT=opc.tcp://localhost:48010 go test ./internal/opcua -run Method -count=1
```

## Buildable implementation slices

### Slice 1 — [#37 Discover and inspect Method Nodes](https://github.com/kevin-rieck/project-cobalt/issues/37)

**Depends on:** none.

- Add `ParentNodeID` to browsed Address Nodes; preserve `(ParentNodeID, NodeID)` identity for Method Nodes in Address Space Search so shared Methods keep each owner.
- Add Method metadata models and `Client.ReadMethodDetails`.
- Decode Method attributes and argument properties, including honest partial/failure behavior.
- Add `App.GetMethodDetails` and focused client/App tests.
- Add the target-server metadata integration test.

**Acceptance:** A Method Node from tree or search carries its Object Node ID, and Wails returns ordered metadata for MethodIO while Read-Only Mode is active.

### Slice 2 — [#38 Execute typed Method calls behind safety gates](https://github.com/kevin-rieck/project-cobalt/issues/38)

**Depends on:** [#37](https://github.com/kevin-rieck/project-cobalt/issues/37).

- Generalize the scalar codec without changing Variable Node Write behavior.
- Add `Client.CallMethod` and result projection.
- Add `App.CallMethod` with connected, Read-Only, executable, fresh-signature, count, type, and rank checks.
- Add redacted diagnostics and focused client/App tests.
- Extend the target integration test to assert MethodIO returns 42.

**Acceptance:** The backend calls MethodIO with typed `UInt32` inputs only when changes are allowed and returns StatusCode plus raw outputs; bypassing the UI cannot bypass safety checks.

### Slice 3 — [#39 Build the Method Call panel](https://github.com/kevin-rieck/project-cobalt/issues/39)

**Depends on:** [#37](https://github.com/kevin-rieck/project-cobalt/issues/37) and [#38](https://github.com/kevin-rieck/project-cobalt/issues/38).

- Add Method Node activation from the tree and Address Space Search.
- Implement loading, metadata, argument entry, review summary, disabled reasons, submission, results, toasts, and reset states.
- Broaden session safety wording from Variable Node Write-only to all OPC UA Server changes.
- Add Playwright coverage for the frontend Wails seam and run frontend checks.

**Acceptance:** An Automation Engineer can inspect MethodIO, enter `20` and `22`, deliberately call it after leaving Read-Only Mode, and see `StatusGood` and `42`; all blocked states explain why.

### Slice 4 — [#40 Validate and document Method calls end to end](https://github.com/kevin-rieck/project-cobalt/issues/40)

**Depends on:** [#37](https://github.com/kevin-rieck/project-cobalt/issues/37), [#38](https://github.com/kevin-rieck/project-cobalt/issues/38), and [#39](https://github.com/kevin-rieck/project-cobalt/issues/39).

- Exercise input-only, output-only, and no-argument target Methods where safe.
- Run all Go, frontend, and opt-in target-server tests.
- Update generated Wails bindings, README capability text/screenshots as appropriate, and the domain glossary with **Method Node** and **Method Call**.
- Record any discovered server/vendor limitations; do not expand v1 DataType scope in this slice.

**Acceptance:** all automated checks pass, the target-server tracer path is documented and repeatable, generated bindings are current, and user-facing/domain documentation matches the shipped behavior.

## Architecture record decision

No ADR is needed for the transport/API shape: it follows existing `internal/opcua.Client` and Wails boundaries. The conservative rule that all Method calls are blocked in Read-Only Mode is product safety policy. Add a short ADR only if implementation work introduces exceptions for non-mutating Methods; that exception mechanism would be a durable architectural decision. Add glossary entries when the feature ships, not during this planning-only issue.
