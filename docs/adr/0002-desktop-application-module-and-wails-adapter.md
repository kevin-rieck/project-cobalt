# Desktop Application Module and Wails Adapter

OPC UA Studio keeps desktop Troubleshooting Session behavior in the platform-neutral `internal/application` module. The Wails `App` translates desktop delivery concerns at the seam and delegates session operations to that module.

## Decision

`internal/application` owns the active OPC UA connection, Saved Connections, Address Space Search, Variable Node Inspection, Watchlist, Session Trend, Read-Only Mode, operation coordination, diagnostics, and typed application events. It accepts OPC UA client creation, Saved Connection persistence, time, logging, event publication, and Address Space Search construction as dependencies. It does not import Wails packages.

The desktop `App` remains responsible for Wails lifecycle/event translation and native Client Certificate/private-key dialogs. Its public Wails request and response models remain stable while it translates to application models.

The application module is the primary desktop behavioral test seam. Wails-adapter tests focus on translating platform events and native capabilities.

## Consequences

Session-safety rules and OPC UA behavior have one implementation within the desktop target. ADR-0001's conservative Rate-Limited Browsing behavior remains in that implementation.

The Wails adapter adds a small amount of model and event translation. It must not recreate connection, mutation, or Read-Only safety behavior.

The browser product is maintained independently in `kevin-rieck/ostudio-web`; this desktop repository does not import or package its runtime code.

## Rejected alternatives

- Keep the Wails-bound `App` as the application core: this would couple behavioral tests and session safety to desktop runtime calls.
- Put Wails details into the application module: this would couple the primary behavioral seam to the delivery mechanism.
- Recreate session safety in the Wails adapter: this would split invariants across two desktop modules and make transitions harder to verify.
