# Single-Controller Web Runtime

The browser delivery target runs one embedded OPC UA Studio process per container. It owns one active OPC UA connection and Troubleshooting Session.

## Decision

The web runtime has one fixed administrator identity and permits one authenticated browser to hold the controller lease. Other authenticated browsers may observe application state, but every application state change requires the current lease.

A controller may explicitly take over immediately. Takeover revokes the old lease and restores Read-Only Mode. Controller loss also restores Read-Only Mode; the runtime then starts the bounded disconnect grace period. The same browser may recover its own brief interruption only when no takeover occurred, and recovery remains Read-Only Mode.

The application publishes authoritative snapshots and sequenced events; controller, authentication, and active Troubleshooting Session state are in memory only. Container restart invalidates that state. The web runtime does not automatically reconnect to an OPC UA Server.

## Consequences

The single-controller model keeps Variable Node Writes and Method Calls attributable to one Automation Engineer and gives controller loss a fail-safe state. It is intentionally a single-operator deployment, not an OPC UA gateway or shared service.

## Rejected alternatives

- Multiple independent controller roles or simultaneous writers: they complicate mutation ownership without a v1 operational need.
- Multiple replicas for one logical instance: they would require distributed controller, event, and active-session coordination.
- Automatic OPC UA reconnect: it could silently resume a changed industrial state after ownership or connectivity loss.
