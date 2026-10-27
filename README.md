# dns-checker

[![tag](https://img.shields.io/github/v/tag/andrewheberle/dns-checker)](https://github.com/andrewheberle/dns-checker/-/tags)
[![LICENSE](https://img.shields.io/badge/license-MIT-blue)](https://github.com/andrewheberle/dns-checker/-/blob/main/LICENSE)

This can be used as a sidecar to provide health checks for a DNS pod in Kubernetes.

## Command-Line Options

* `--listen`: Listen address (string)
* `--server`: DNS server to query (string)
* `--lookup`: Hostname to lookup (string)

All command line options may be specified as environment variables in the form of `DNS_<option>`
