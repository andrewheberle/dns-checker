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
