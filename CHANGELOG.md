# Changelog

## [1.2.1](https://github.com/andrewheberle/dns-checker/compare/v1.2.0...v1.2.1) (2026-11-02)


### Bug Fixes

* **deps:** update cgr.dev/chainguard/static:latest docker digest to fc42af3 ([#51](https://github.com/andrewheberle/dns-checker/issues/51)) ([a73411f](https://github.com/andrewheberle/dns-checker/commit/a73411fc28a7627e86491538e70aadca27019813))
* **deps:** update module golang.org/x/net to v0.62.0 ([#49](https://github.com/andrewheberle/dns-checker/issues/49)) ([df235a4](https://github.com/andrewheberle/dns-checker/commit/df235a40cb52137b005ecd3369bdb94c9d58a026))

## [1.2.0](https://github.com/andrewheberle/dns-checker/compare/v1.1.0...v1.2.0) (2026-10-27)


### Features

* validate --server at startup and default the port to 53 ([#45](https://github.com/andrewheberle/dns-checker/issues/45)) ([4824ddf](https://github.com/andrewheberle/dns-checker/commit/4824ddfc495895f6a9d088aa3790705f8baf6663))


### Bug Fixes

* query the target server directly instead of via net.Resolver ([#46](https://github.com/andrewheberle/dns-checker/issues/46)) ([b5d9ffd](https://github.com/andrewheberle/dns-checker/commit/b5d9ffd9ffb86efff84d994023303c1bd10fa856))
* read DNS_* environment variables again ([#42](https://github.com/andrewheberle/dns-checker/issues/42)) ([bf7841e](https://github.com/andrewheberle/dns-checker/commit/bf7841e9b4f2fd0d0f46b254a665c5c3668d5900))
