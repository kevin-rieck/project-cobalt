# Independent TypeScript Stack for the Web Target

OPC UA Studio's web target requires a browser UI and a server-side OPC UA client. The existing desktop target remains a Wails application with a Svelte frontend and a Go application module.

## Decision

The web target uses an independent TypeScript stack under `web/`:

- `web/apps/client` is a React/Vite browser application;
- `web/apps/server` is the Node.js composition root and same-origin HTTP/SSE delivery adapter;
- `web/packages/application` owns the web target's platform-neutral Troubleshooting Session behavior;
- `web/packages/node-opcua-adapter` is the only module that imports `node-opcua`; and
- `web/packages/contracts` contains generated types for the private, versioned web transport contract.

`node-opcua` runs only in the Node.js process. The browser never opens an OPC UA connection and never receives Client Certificate private keys. The Node.js process serves the built React assets and `/api/v1` from the same origin.

The desktop and web targets do not call each other or share runtime implementation code. They share product terminology, documented safety invariants, versioned transport schemas where applicable, and language-neutral conformance fixtures. Both implementations must pass the applicable conformance tests.

The TypeScript application module is the web target's primary behavioral test seam. It accepts OPC UA client creation, Saved Connection persistence, time, logging, and event publication as dependencies. It does not import React, the HTTP framework, Node.js filesystem modules, or `node-opcua`.

Before relying on `node-opcua`, an executable capability spike must establish endpoint selection, certificate handling, disabled automatic reconnect, continuation handling, subscriptions, Method metadata, operation timeout semantics, and exact Int64/UInt64 representation.

## Consequences

The desktop application and its build remain independent and unchanged by ordinary web development. The web target can use React and the Node.js OPC UA ecosystem without introducing a Go-to-Node bridge or bundling server dependencies into the browser.

Troubleshooting Session and safety behavior is implemented twice. This creates drift risk, so safety invariants and cross-stack fixtures are release gates rather than optional documentation. Fixes are applied independently to each stack when an invariant applies to both.

The web image contains a Node.js runtime rather than a Go web binary. The Node.js process owns authentication, controller leases, browser liveness renewal, HTTP/SSE delivery, OPC UA sessions, persistence adapters, and shutdown.

## Supersedes

This decision supersedes ADR-0002 for the web target. ADR-0002 continues to describe the desktop application's internal organization. ADR-0001, ADR-0003, and ADR-0004 remain in force.

## Rejected alternatives

- Share the Go application module through a Go HTTP server: this does not use `node-opcua` and keeps the web implementation tied to the desktop runtime.
- Run `node-opcua` in React: browser runtimes cannot safely provide the Node.js networking, certificate, and private-key capabilities required by OPC UA.
- Add a Go-to-Node bridge behind the web server: this creates two server runtimes and a second internal transport without reducing the safety-critical porting work enough to justify the operational complexity.
- Migrate the desktop frontend to React: the web target does not require a desktop rewrite, and keeping desktop unchanged limits regression risk.
