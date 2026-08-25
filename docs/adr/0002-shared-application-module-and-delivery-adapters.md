# Shared Application Module and Delivery Adapters

> **Status:** Superseded for the web target by [ADR-0005](0005-independent-web-typescript-stack.md). This decision remains the description of the Wails desktop application's internal organization.

OPC UA Studio keeps desktop Troubleshooting Session behavior in the platform-neutral `internal/application` module. The Wails desktop `App` translates its delivery concerns at the seam and delegates session operations to that module.

## Decision

`internal/application` owns the active OPC UA connection, Saved Connections, Address Space Search, Variable Node Inspection, Watchlist, Session Trend, Read-Only Mode, operation coordination, diagnostics, and typed application events. It accepts OPC UA client creation, Saved Connection persistence, time, logging, event publication, and Address Space Search construction as dependencies. It does not import Wails or HTTP packages.

The desktop `App` remains responsible for Wails lifecycle/event translation and native Client Certificate/private-key dialogs. Its public Wails request and response models remain stable while it translates to the application models.

The application module remains the primary behavioral test seam for the desktop target. Delivery-adapter tests focus on translating platform events and native capabilities.

## Consequences

Session-safety rules and OPC UA behavior have one implementation within the desktop target. ADR-0001's conservative Rate-Limited Browsing behavior remains in that implementation.

The Wails adapter adds a small amount of model/event translation. It must not recreate connection, mutation, or Read-Only safety behavior. The independent web implementation maintains parity through the invariants and conformance approach established by ADR-0005.

## Rejected alternatives

- Keep the Wails-bound `App` as the application core: this would make browser delivery duplicate or bypass session safety behavior.
- Give desktop and web independent session implementations without a conformance contract: this would let safety and recovery rules diverge across delivery targets. ADR-0005 accepts independent implementations with explicit invariants and cross-stack fixtures.
- Put Wails or HTTP details into the shared module: this would couple the primary behavioral seam to a delivery mechanism and prevent reuse.
