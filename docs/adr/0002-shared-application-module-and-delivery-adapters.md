# Shared Application Module and Delivery Adapters

OPC UA Studio keeps Troubleshooting Session behavior in the platform-neutral `internal/application` module. The Wails desktop `App` and the future HTTP delivery adapter translate their respective transport concerns at the boundary and delegate session operations to that module.

## Decision

`internal/application` owns the active OPC UA connection, Saved Connections, Address Space Search, Variable Node Inspection, Watchlist, Session Trend, Read-Only Mode, operation coordination, diagnostics, and typed application events. It accepts OPC UA client creation, Saved Connection persistence, time, logging, event publication, and Address Space Search construction as dependencies. It does not import Wails or HTTP packages.

The desktop `App` remains responsible for Wails lifecycle/event translation and native Client Certificate/private-key dialogs. Its public Wails request and response models remain stable while it translates to the application models. A future HTTP adapter will own its HTTP, authentication, controller-lease, and SSE concerns at the same boundary.

The application module remains the primary behavioral test seam. Delivery-adapter tests focus on translating their platform events and native capabilities.

## Consequences

Session-safety rules and OPC UA behavior have one implementation for desktop and web delivery targets. ADR-0001's conservative Rate-Limited Browsing behavior remains in that shared implementation.

The adapters add a small amount of model/event translation. They must not recreate connection, mutation, or Read-Only safety behavior.

## Rejected alternatives

- Keep the Wails-bound `App` as the application core: this would make browser delivery duplicate or bypass session safety behavior.
- Give desktop and web independent session implementations: this would let safety and recovery rules diverge across delivery targets.
- Put Wails or HTTP details into the shared module: this would couple the primary behavioral seam to a delivery mechanism and prevent reuse.
