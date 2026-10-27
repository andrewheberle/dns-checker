# DNS Checker

A sidecar that reports whether a DNS server is healthy, so an orchestrator such as Kubernetes can act on it.

## Language

**Probe**:
A single health check of the target server: one attempt to resolve the lookup host through it.
_Avoid_: Test, ping

**Target server**:
The DNS server whose health is being reported.
_Avoid_: Upstream, resolver, DNS service

**Lookup host**:
The hostname resolved during a probe to prove the target server is answering.
_Avoid_: Query, test domain
