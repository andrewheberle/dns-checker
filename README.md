# dns-checker

[![codecov](https://codecov.io/gh/andrewheberle/dns-checker/graph/badge.svg?token=VZM09OWOXA)](https://codecov.io/gh/andrewheberle/dns-checker)
[![tag](https://img.shields.io/github/v/tag/andrewheberle/dns-checker)](https://github.com/andrewheberle/dns-checker/-/tags)
[![LICENSE](https://img.shields.io/badge/license-MIT-blue)](https://github.com/andrewheberle/dns-checker/-/blob/main/LICENSE)

This can be used as a sidecar to provide health checks for a DNS pod in Kubernetes.

## Health Check

`GET /healthz` sends an A query for the lookup host directly to the target server over UDP, followed by an AAAA query if there were no A records. It returns `200` if the server replies `NOERROR` with an address or a CNAME for the lookup host, and `500` otherwise. CNAMEs are not followed, and `/etc/hosts` and search domains are never used.

## Command-Line Options

* `--listen`: Listen address (string)
* `--server`: DNS server to query, as `host:port` (port defaults to 53 if omitted) (string)
* `--lookup`: Hostname to lookup, always treated as fully qualified (string)

All command line options may be specified as environment variables in the form of `DNS_<option>`

## Example: Sidecar for dnsmasq

This runs [dnsmasq](https://thekelleys.org.uk/dnsmasq/doc.html) in a Pod with dns-checker as a sidecar. The two containers share the Pod's network, so dns-checker probes dnsmasq on `127.0.0.1:53`.

dnsmasq answers `healthcheck.internal` from a local record, so the check shows dnsmasq itself is working rather than depending on its upstream resolvers.

The probes are set on the dnsmasq container, so if dnsmasq stops answering, the kubelet restarts dnsmasq and stops sending it traffic. They call `/healthz` on port 8080, which the dns-checker image listens on by default. `timeoutSeconds` is longer than dns-checker's own 4 second timeout.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dnsmasq
data:
  healthcheck.conf: |
    host-record=healthcheck.internal,192.0.2.1
---
apiVersion: v1
kind: Pod
metadata:
  name: dnsmasq
  labels:
    app: dnsmasq
spec:
  containers:
    - name: dnsmasq
      image: dockurr/dnsmasq:2.93
      ports:
        - name: dns
          containerPort: 53
          protocol: UDP
        - name: dns-tcp
          containerPort: 53
          protocol: TCP
      volumeMounts:
        - name: config
          mountPath: /etc/dnsmasq.d
      livenessProbe:
        httpGet:
          path: /healthz
          port: 8080
        periodSeconds: 10
        timeoutSeconds: 5
        failureThreshold: 3
      readinessProbe:
        httpGet:
          path: /healthz
          port: 8080
        periodSeconds: 10
        timeoutSeconds: 5
    - name: dns-checker
      image: ghcr.io/andrewheberle/dns-checker:v1.1.0 # x-release-please-version
      args:
        - --server=127.0.0.1:53
        - --lookup=healthcheck.internal
      ports:
        - name: health
          containerPort: 8080
  volumes:
    - name: config
      configMap:
        name: dnsmasq
```
