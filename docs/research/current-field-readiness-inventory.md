# OPC UA Studio v0.2 field-readiness inventory

**Status:** current-state inventory  
**Access date:** 2026-08-13  
**Scope:** the [v0.2 field-readiness map](https://github.com/kevin-rieck/project-cobalt/issues/60), not implementation work

This inventory records what the repository currently provides. It does not claim that the behaviors below have passed a field-readiness gate. The compatible-server recommendation is recorded separately in [the compatibility matrix](./opc-ua-server-compatibility-matrix.md).

## Current behavior

| Area | Present behavior | Evidence |
|---|---|---|
| Connection and identity | The connection flow discovers endpoints and supports Anonymous and Username identities. Secure endpoints require a Client Certificate and private key; the selected server certificate thumbprint is displayed and retained with a Saved Connection. Passwords are not persisted. | `app.go`; `internal/opcua/client.go`; `internal/connections/store.go`; `frontend/src/App.svelte`; `README.md` |
| Browsing and Address Space Search | The app browses the Address Space and performs rate-limited, shallow metadata indexing for Address Space Search. It preserves the existing connection when a replacement connection fails, and clears session-local search data after disconnect or successful reconnect. | `internal/search/`; `app.go`; `app_test.go`; `docs/adr/0001-hybrid-address-space-search.md` |
| Variable Node Inspection and subscriptions | Variable Node Inspection presents a Live Value, status, timestamp, data type, and metadata. The Watchlist and Session Trend retain observed updates only for the current Troubleshooting Session. | `internal/session/`; `app.go`; `frontend/src/App.svelte`; `README.md` |
| Variable Node Write | The backend rejects writes while disconnected or in Read-Only Mode and blocks stale, uninspected, unsupported, non-scalar, non-writable, or unavailable values. The UI requires confirmation, reports read-back results, and shows status/range warnings. | `app.go`; `internal/opcua/write.go`; `app_test.go`; `frontend/tests/ui/variable-node-write.spec.ts` |
| Method Calls | Method Nodes are discovered under their owning Object Node. The app reads argument metadata, blocks calls in Read-Only Mode, rereads metadata before calling, validates supported scalar inputs, and renders StatusCode/output arguments. | `internal/opcua/method.go`; `app.go`; `frontend/src/MethodCallPanel.svelte`; `internal/opcua/client_method*_test.go`; `frontend/tests/ui/method-call.spec.ts`; `docs/plans/0028-opc-ua-method-calls.md` |
| Session safety and reconnect | Every successful connection and disconnect restores Read-Only Mode. A failed reconnect retains the existing connection and Address Space Search context; a successful reconnect begins a new session and clears the prior context. | `app.go`; `app_test.go` |
| Diagnostics | The app keeps bounded, timestamped in-memory Diagnostic Log entries, emits new entries to the frontend, and records connection, browse, write, Method, and saved-connection failures. | `app.go`; `app_test.go`; `frontend/src/App.svelte`; `frontend/tests/ui/variable-node-write.spec.ts` |

## Existing tests and fixtures

| Layer | Existing coverage | Limits |
|---|---|---|
| Go unit tests | `go test ./...` covers connection/storage errors, session safety, browse/search behavior, inspection, scalar parsing, writes, Method metadata/calls, and client authentication validation. It passed on 2026-08-13. | These tests use fakes or local seams; they do not prove Wails desktop behavior or interoperability with the v0.2 matrix. |
| Optional live OPC UA tests | `internal/opcua/client_integration_test.go` can discover, connect, browse, inspect Method metadata, and call a known `UInt32 + UInt32` Method on a manually started server. The README documents the Unified Automation C++ Demo Server invocation. | Both endpoint variables are opt-in (`TERMUA_TEST_ENDPOINT` and `TERMUA_METHOD_TEST_ENDPOINT`); CI neither provisions nor runs them. The Method tracer is anonymous over SecurityPolicy None and depends on vendor-demo node IDs. |
| Browser UI tests | 33 Playwright cases cover connection-manager rendering, search navigation, Variable Node Write confirmation/error states, and Method Call states. A full parallel rerun passed on 2026-08-13. | Playwright starts Vite (`frontend/playwright.config.ts`) and substitutes browser-side Wails API responses in the workflow tests. It is not a desktop/package/real-server test. One initial full `npm run ci` run had a one-off timeout in the read-back-mismatch UI case; three focused serial reruns and the subsequent full Playwright rerun passed. |
| Screenshot fixtures | Deterministic screenshot mode powers checked-in README images, and `frontend/scripts/assert-issue-6.mjs` asserts Saved Connection presentation. | Screenshot and assertion scripts are not release gates. |
| Server fixtures | No project-owned OPC UA Server fixture, PKI setup, fault injector, fixture process runner, or recorded compatibility result exists. | The selected open62541 and UA-.NETStandard configurations in the compatibility matrix remain proposed fixture work. |

## Build, release, and packaging

| Area | Present state | Evidence |
|---|---|---|
| Local build | Developers can run `wails dev` and `wails build`; the Wails build directory contains Windows manifest, icon, version metadata, and NSIS installer template. | `README.md`; `build/README.md`; `build/windows/` |
| Windows installer support | The NSIS template can create an architecture-checked installer, install WebView2, create shortcuts, and uninstall. Its comments document `wails build --target windows/amd64 --nsis`. | `build/windows/installer/project.nsi` |
| CI | The frontend workflow runs `npm run ci` only for frontend-path changes. The tag release workflow builds Linux, Windows amd64, and macOS assets and publishes checksums. | `.github/workflows/frontend-security.yml`; `.github/workflows/release.yml`; `frontend/package.json` |
| Published Windows artifact | The tag workflow stages `build/bin/opcua-studio.exe` as a raw executable. | `.github/workflows/release.yml` |

## Documentation present

- `README.md` documents the current product workflow, supported scalar Method input boundary, Saved Connections, local development/build commands, screenshot regeneration, and the optional Unified Automation Method tracer.
- `CONTEXT.md` defines the project vocabulary, including Client Certificate, Read-Only Mode, Troubleshooting Session Recovery, and Diagnostic Report.
- `docs/adr/0001-hybrid-address-space-search.md` records the hybrid Address Space Search decision.
- `docs/plans/0028-opc-ua-method-calls.md` records the Method Call design and its opt-in integration-test rationale.

## Concrete gaps for later decisions

1. **No reproducible compatibility gate.** The repository has no checked-in open62541 or UA-.NETStandard fixture, pinned build/provisioning scripts, isolated PKI, test evidence, or CI job for the [selected matrix](./opc-ua-server-compatibility-matrix.md).
2. **Secure trust behavior is not proven.** Requiring certificate/key paths and displaying a server thumbprint does not establish client-side server-certificate validation, server-side application-certificate trust rejection/provisioning, or trust-store lifecycle behavior.
3. **Troubleshooting Session Recovery is only partial current behavior.** Existing reconnect safety tests protect session state, but there is no specified recovery workflow, fault-injection coverage, or evidence that an interrupted Troubleshooting Session can be deliberately continued without resuming mutation.
4. **No desktop or clean-machine acceptance test.** UI tests stub the Wails bridge, and no automated scenario starts a real OPC UA Server, drives the Windows desktop application, or checks a package on a clean Windows environment.
5. **Release is not validation-gated.** A tag can publish after a Wails build and checksum generation without Go tests, frontend tests, fixture matrix results, installer smoke testing, or diagnostic redaction/export checks.
6. **The release does not publish the prepared Windows installer.** Although NSIS/WebView2 support exists, the release workflow publishes only the raw Windows executable. Installer, WebView2, shortcut, uninstall, migration, and rollback behavior have no acceptance evidence.
7. **Diagnostic Report contract is unfulfilled.** The glossary calls for a sanitized, exportable Diagnostic Report, while the product exposes only in-memory, viewable Diagnostic Logs. There is no export API, sanitization/redaction contract, or operational troubleshooting documentation/evidence.
8. **Supported envelope is not decided.** The current README documents a v1 scalar Method boundary, but v0.2 still needs explicit Windows versions/architectures, security-policy/trust rules, scalar/browse/subscription limits, required evidence, known-limitations treatment, and release-blocking rules.
9. **Saved Connection upgrade safety is untested.** Storage handles corrupt JSON diagnostically and omits passwords, but no current migration, installer upgrade, rollback, or clean-machine persistence scenario is documented or automated.

## Inputs to the next decisions

The later compatibility, resilience, diagnostics, packaging, and release-acceptance decisions should treat the above gaps as planning inputs rather than infer current support from unit or browser tests. In particular, the compatibility matrix supplies the candidate server fixtures, while this inventory identifies the missing harness, desktop validation, diagnostic-report, and Windows distribution decisions needed to turn that matrix into a developer-run release decision.
