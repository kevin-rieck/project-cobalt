# Reproducible OPC UA Server compatibility matrix for OPC UA Studio v0.2

**Status:** research recommendation

**Access date:** 2026-08-13

**Scope:** developer-run release gating on Windows

## Recommendation

Use a **two-implementation, two-configuration matrix**:

1. a project-owned, deterministic server fixture built with **open62541 v1.5.6** for broad functional coverage over an anonymous `SecurityPolicy None` endpoint; and
2. a derived configuration of the OPC Foundation **UA-.NETStandard Console Reference Server 1.5.378.156** for secure-channel, username, application-certificate, and trust-failure coverage over `Basic256Sha256` with `SignAndEncrypt`.

At fixture implementation time, pin both the release and resolved commit, retain build/configuration scripts, and record artifact hashes. The candidate source pins researched here are:

- open62541 `v1.5.6`, tag target `cd69ed6f6a4d46966f67794e23a7b7b331f7d402` ([release](https://github.com/open62541/open62541/releases/tag/v1.5.6), [source](https://github.com/open62541/open62541/tree/v1.5.6));
- UA-.NETStandard `1.5.378.156`, tag target `083dbe2ccf0a43939164cc0bed0c9e79f2e60786` ([release](https://github.com/OPCFoundation/UA-.NETStandard/releases/tag/1.5.378.156), [source](https://github.com/OPCFoundation/UA-.NETStandard/tree/1.5.378.156)).

These are recommendations, not passing results. Neither fixture has yet been built or exercised by OPC UA Studio.

## Terminology boundary: Client Certificate

The project glossary defines **Client Certificate** as the application-instance certificate presented by OPC UA Studio for signed or encrypted communication. This is distinct from an OPC UA `X509IdentityToken`, which identifies a user during session activation.

The release-gating matrix therefore tests OPC UA Studio's **application certificate** and private key, including server-side rejection before that application certificate is trusted and success after explicit trust provisioning. X.509 user identity is not part of the current product contract or this minimum matrix. OPC UA distinguishes application authentication from user identity ([Part 4 §6.1.4](https://reference.opcfoundation.org/Core/Part4/v105/docs/6.1.4), [Part 4 §7.40](https://reference.opcfoundation.org/Core/Part4/v105/docs/7.40)).

## Selected matrix

| Gate | Implementation and configuration | Required coverage |
|---|---|---|
| **A — deterministic functional fixture** | open62541, project-owned source fixture; loopback endpoint with `SecurityPolicy None`, `MessageSecurityMode None`, and Anonymous identity only | Discovery, connect, browse, read, subscriptions, writable and read-only Variable Nodes, deterministic Method Calls, and supported scalar values |
| **B — secure interoperability fixture** | UA-.NETStandard Console Reference Server; isolated configuration and PKI stores; `Basic256Sha256` + `SignAndEncrypt`; Username identity; Anonymous disabled | Client rejection of an untrusted server certificate, success after client-side trust provisioning, server rejection of an untrusted OPC UA Studio application certificate, success after server-side trust provisioning, wrong-password rejection, correct-password connection, then browse/read/subscribe/write/Method smoke tests |

### Why this is the smallest defensible matrix

A single implementation can mask matching assumptions between a client and server. Two independent, non-Go stacks provide basic interoperability evidence while remaining freely obtainable, source-buildable, pinnable, and automatable on Windows.

open62541 provides a compact C server that the project can shape into a deterministic fixture. Its official source includes Windows/CMake build support and server examples for Variables and Methods ([build documentation](https://www.open62541.org/doc/v1.5.0/building.html), [Variable example](https://github.com/open62541/open62541/blob/v1.5.6/examples/tutorial_server_variable.c), [Method example](https://github.com/open62541/open62541/blob/v1.5.6/examples/tutorial_server_method.c)).

The OPC Foundation reference implementation supplies an independent .NET stack and a Windows-friendly console server. Its pinned configuration exposes `Basic256Sha256`, username token policies, application-certificate stores, trusted-peer stores, and rejected-certificate stores ([configuration](https://github.com/OPCFoundation/UA-.NETStandard/blob/1.5.378.156/Applications/ConsoleReferenceServer/Quickstarts.ReferenceServer.Config.xml)). Its server source validates usernames and certificate identity tokens, although X.509 user identity remains outside this matrix ([server implementation](https://github.com/OPCFoundation/UA-.NETStandard/blob/1.5.378.156/Applications/Quickstarts.Servers/ReferenceServer/ReferenceServer.cs)).

Adding a third implementation would increase runtime and fixture maintenance without covering a requirement absent from these two configurations.

## Fixture contracts

### Gate A: deterministic open62541 fixture

The project should own a small server executable rather than rely on changing upstream examples. It must provide:

- a stable Object hierarchy with explicit namespace URI and string NodeIds;
- enough references to exercise hierarchical browsing and continuation behavior;
- readable scalar Variable Nodes for every scalar type claimed by v0.2;
- one writable scalar Variable with a known initial value and deterministic read-back;
- one read-only Variable whose write produces a non-Good StatusCode;
- one deterministic changing Variable for bounded DataChange subscription tests;
- input/output, output-only, and no-argument Methods, including `UInt32 + UInt32 -> UInt32`;
- fixed startup state, an explicit readiness signal, captured logs, and clean shutdown.

The official source and examples establish that such a fixture can be authored; these exact semantics remain project work and must be validated.

### Gate B: secure UA-.NETStandard fixture

Use a checked-in derived configuration and a process-local PKI root recreated for each test run. Require:

- only the intended `Basic256Sha256` + `SignAndEncrypt` endpoint for this gate;
- Username identity with a deterministic test account and Anonymous disabled;
- automatic trust disabled;
- distinct application certificates for OPC UA Studio and the OPC UA Server;
- an empty OPC UA Studio trust store for the first attempt, which must reject the server certificate;
- explicit provisioning of that server certificate or its test issuer, after which client-side server validation succeeds;
- an empty server trusted-peer store for the next attempt, which must reject OPC UA Studio's application certificate;
- explicit provisioning of OPC UA Studio's application certificate or its test issuer, after which application authentication succeeds;
- wrong-password failure followed by correct-password success;
- browse/read/subscribe/write/Method smoke checks after authentication.

The derived server may need a small test-only validation override so credentials and rejection behavior are deterministic. The pinned upstream defaults must not be treated as the fixture contract without verification.

Do not read or modify machine-wide Windows certificate stores. Use fixed test DNS/ApplicationUri values, scripted test-only certificates, and isolated directories. Capture the expected certificate-validation error, certificate thumbprints, configuration hash, and trust-store paths in test evidence.

## Coverage map

| Requirement | Gate and assertion |
|---|---|
| Anonymous + SecurityPolicy None | **A:** exact endpoint selected; anonymous session activated |
| Username over a secure channel | **B:** wrong password rejected; correct credentials accepted only on `SignAndEncrypt` |
| Client Certificate | **B:** the server rejects OPC UA Studio's untrusted application certificate, then accepts it after explicit trust provisioning |
| Server-certificate verification and trust failures | **B:** OPC UA Studio rejects the initially untrusted server certificate, then accepts it after explicit trust provisioning |
| Browsing | **A**, smoke in **B:** expected stable nodes and continuation behavior |
| Subscriptions | **A**, smoke in **B:** bounded DataChanges with Good status |
| Variable Node Writes | **A**, smoke in **B:** writable value changes and reads back; read-only write fails |
| Method Calls | **A**, smoke in **B:** known signatures and deterministic StatusCode/output |

## Current-product gaps exposed by the matrix

This selection does not imply current support:

- OPC UA Studio currently supports Anonymous and Username user identities, not X.509 user identity. That is consistent with the matrix.
- Secure endpoints already require a Client Certificate and private key, but the later current-state inventory must verify the exact server-side application trust behavior.
- the saved server thumbprint is currently descriptive data rather than a complete client trust-store or certificate-validation workflow;
- there is no automated fixture provisioning, isolated PKI harness, or recorded matrix result today;
- the existing Unified Automation integration tracer covers only an optional anonymous Method scenario.

These are inputs to later Wayfinder decisions, not reasons to expand this research ticket into implementation.

## Rejected alternatives

### Unified Automation C++ Demo Server

Retain the existing Method tracer as optional manual interoperability evidence, but do not gate v0.2 on a vendor demo binary whose exact historical installer, unattended setup, redistribution rights, and address-space stability are not controlled by this repository ([official product downloads](https://www.unified-automation.com/downloads/opc-ua-servers.html)).

### Prosys OPC UA Simulation Server

Useful for exploratory testing, but its vendor-distributed desktop product and installation lifecycle provide less fixture control than the selected source-buildable servers ([official product page](https://prosysopc.com/products/opc-ua-simulation-server/)).

### Eclipse Milo and node-opcua

Both are legitimate open-source implementations, but they add Java or Node.js fixture maintenance without unique v0.2 requirement coverage after open62541 and UA-.NETStandard ([Eclipse Milo source and EPL-2.0 license](https://github.com/eclipse-milo/milo/blob/main/LICENSE.md), [node-opcua source](https://github.com/node-opcua/node-opcua), [node-opcua MIT license](https://github.com/node-opcua/node-opcua/blob/master/LICENSE)). Keep either as a future tie-breaker for stack-specific defects.

Commercial products, expiring trials, account-bound activation, and unavailable historical installers are unsuitable for mandatory reproducible gates.

## Risks that remain

| Severity | Risk | Required treatment |
|---|---|---|
| High | Upstream sample defaults may accept broader identities than intended. | Own derived configuration/validation and prove every negative case. |
| High | PKI residue can invalidate trust tests. | Fresh process-local stores; no auto-accept; deterministic cleanup. |
| High | Application certificates can be confused with X.509 user identity. | Preserve the glossary distinction in product copy, fixtures, and test names. |
| Medium | The selected Windows builds are not yet automated in this repository. | Perform a clean-Windows fixture spike before making gates mandatory. |
| Medium | Floating dependencies or expired certificates make results irreproducible. | Pin source/toolchains/dependencies and decide scripted certificate renewal. |
| Medium | Subscription timing can be flaky. | Deterministic server counter, bounded retries, and finite tolerant timeouts. |
| Medium | Redistributing cached open62541 binaries may create MPL-2.0 obligations beyond retaining notices. | Prefer build-from-source fixtures and obtain project-owner review before redistribution ([MPL-2.0 license](https://github.com/open62541/open62541/blob/v1.5.6/LICENSE)). |

UA-.NETStandard is supplied under the OPC Foundation MIT license ([license](https://github.com/OPCFoundation/UA-.NETStandard/blob/1.5.378.156/LICENSE.txt)).

## Reproducibility acceptance for the eventual fixtures

A clean supported Windows environment must be able to:

1. fetch and verify the pinned sources without accounts, activation, or expiring trials;
2. build both servers non-interactively with pinned prerequisites;
3. create isolated configuration, credentials, and PKI without a GUI;
4. start each server, detect readiness, capture logs, and shut it down;
5. execute every positive and negative assertion from fresh server and trust state;
6. reset mutable Address Space state between runs; and
7. remove temporary private keys, trust stores, processes, and ports after success or failure.

## Decisions deferred to the supported compatibility envelope

The later compatibility-envelope ticket must decide:

- supported Windows versions and architectures;
- final toolchain/dependency pins and artifact retention;
- exact security policies beyond required `None` and `Basic256Sha256`;
- leaf pinning versus issuer trust, hostname/SAN and ApplicationUri checks, validity periods, chains, and revocation expectations;
- exact scalar DataTypes, browse size/depth/continuation limits, subscription timing, and Method signatures;
- whether a future product capability should add X.509 user identity;
- evidence format, release-blocking rules, and upstream pin-update policy.

## Conclusion

Select open62541 plus the UA-.NETStandard Console Reference Server. Their two controlled configurations cover every release-gating scenario named by this ticket while preserving the project meaning of Client Certificate. Keep the Unified Automation server as optional manual comparison evidence, not a reproducibility dependency.
