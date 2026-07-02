# Changelog

## [0.2.2](https://github.com/pacorreia/fifa-wc-2026-card-trading/compare/v0.2.1...v0.2.2) (2026-07-02)


### Bug Fixes

* update Dockerfile to use Go 1.24 matching go.mod requirement ([0d4d5e4](https://github.com/pacorreia/fifa-wc-2026-card-trading/commit/0d4d5e424a775ca7fec07604632b69f7ba9b4ea6))

## [0.2.0](https://github.com/pacorreia/fifa-wc-2026-card-trading/compare/v0.1.0...v0.2.0) (2026-07-01)


### Features

* add release workflows for docker and helm publishing ([eaf0f96](https://github.com/pacorreia/fifa-wc-2026-card-trading/commit/eaf0f9630d20b059b0ab01601bb79794359152de))
* implement production-ready MVP for WC 2026 Panini sticker trading app ([ac6a1ae](https://github.com/pacorreia/fifa-wc-2026-card-trading/commit/ac6a1aecc57f639e9aab56506ec2c3267edd5f8a))
* implement WC 2026 Panini sticker trading MVP (Go + WebSocket + Helm) ([f0f1cdf](https://github.com/pacorreia/fifa-wc-2026-card-trading/commit/f0f1cdf5c3d555d1e5c844573f7a857e06c4a22d))


### Bug Fixes

* address all PR review issues (hub race, events error, auth atomicity, trade privacy, frontend, helm) ([3bd9618](https://github.com/pacorreia/fifa-wc-2026-card-trading/commit/3bd9618fb8c77c21ba8b68920ece8b088e484dfc))
* harden forced release tag resolution ([0fc3b85](https://github.com/pacorreia/fifa-wc-2026-card-trading/commit/0fc3b85d8573dfd5092df2ede77283ce229099fd))
* swap shutdown/close order in ReadPump defer; log internal errors in GetCollection ([147a184](https://github.com/pacorreia/fifa-wc-2026-card-trading/commit/147a184b54c2640e3376bdee19b3a516f7a0a275))
* verify forced releases before publish jobs ([312a499](https://github.com/pacorreia/fifa-wc-2026-card-trading/commit/312a49998fd1f512a07b760e24f181eaab4c6dd1))
