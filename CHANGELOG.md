# Changelog

## [1.2.0](https://github.com/andrewheberle/dns-checker/compare/v1.1.0...v1.2.0) (2026-10-27)


### Features

* validate --server at startup and default the port to 53 ([#45](https://github.com/andrewheberle/dns-checker/issues/45)) ([4824ddf](https://github.com/andrewheberle/dns-checker/commit/4824ddfc495895f6a9d088aa3790705f8baf6663))


### Bug Fixes

* query the target server directly instead of via net.Resolver ([#46](https://github.com/andrewheberle/dns-checker/issues/46)) ([b5d9ffd](https://github.com/andrewheberle/dns-checker/commit/b5d9ffd9ffb86efff84d994023303c1bd10fa856))
* read DNS_* environment variables again ([#42](https://github.com/andrewheberle/dns-checker/issues/42)) ([bf7841e](https://github.com/andrewheberle/dns-checker/commit/bf7841e9b4f2fd0d0f46b254a665c5c3668d5900))
