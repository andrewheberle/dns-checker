# dns-checker

[![Go Report Card](https://goreportcard.com/badge/github.com/andrewheberle/dns-checker?logo=go&style=flat-square)](https://goreportcard.com/report/github.com/andrewheberle/dns-checker)

This can be used as a sidecar to provide health checks for a DNS pod in Kubernetes.

## Command-Line Options

* `--listen`: Listen address (string)
* `--server`: DNS server to query (string)
* `--lookup`: Hostname to lookup (string)

All command line options may be specified as environment variables in the form of `DNS_<option>`
