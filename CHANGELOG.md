# Changelog

## [1.8.0](https://github.com/bmf-san/gogocoin/compare/v1.7.0...v1.8.0) (2026-05-29)


### Features

* **backtest:** add shared backtest engine and CLI ([a6750dd](https://github.com/bmf-san/gogocoin/commit/a6750dd8e7484c98c7a1eff46f854c1164fef95d))
* **backtest:** add shared backtest engine and CLI ([2ae96c1](https://github.com/bmf-san/gogocoin/commit/2ae96c16c2f0402e9c6e15163409f73e3bd6b418))
* **config:** add trading.observe_symbols for data-collection-only subscriptions ([#107](https://github.com/bmf-san/gogocoin/issues/107)) ([97ee6bd](https://github.com/bmf-san/gogocoin/commit/97ee6bd05f8e2b5a8d957cdab1645f146e7b2da5))
* **web:** add per-symbol realized pnl view ([d85492a](https://github.com/bmf-san/gogocoin/commit/d85492a0d86d1b9d8ef84c8416fe63ff1f8ff49c))
* **web:** add per-symbol realized PnL view ([264c484](https://github.com/bmf-san/gogocoin/commit/264c48438d1edfae9fa172ed51d9fb148c225aed))
* **web:** align dashboard with exchange-style pnl and positions ([92399b7](https://github.com/bmf-san/gogocoin/commit/92399b764bbf91617b6a7bd9dd36a11218c4a587))
* **web:** align dashboard with exchange-style pnl and positions ([c399371](https://github.com/bmf-san/gogocoin/commit/c39937107b4948702aba48a5a92f23f76f33a440))
* **web:** align dashboard with exchange-style pnl and positions ([#112](https://github.com/bmf-san/gogocoin/issues/112)) ([e7687be](https://github.com/bmf-san/gogocoin/commit/e7687bebfa7c52f8ee774734e98de5619ef242d4))
* **worker:** bar aggregation for strategy worker ([#110](https://github.com/bmf-san/gogocoin/issues/110)) ([d6f7020](https://github.com/bmf-san/gogocoin/commit/d6f70208e7904ed295e2269029fd8c0438c0f85b))


### Bug Fixes

* **ci:** sync openapi generated code and example sqlite3 checksum ([c360588](https://github.com/bmf-san/gogocoin/commit/c36058853d64be3064fff2fd6eef1ca4685ce379))
* **example:** sync go.sum for docker build ([0796a35](https://github.com/bmf-san/gogocoin/commit/0796a35f0e01160fc828ddb16df459b716f648b2))
* **example:** sync go.sum for docker build ([025195a](https://github.com/bmf-san/gogocoin/commit/025195a64f029a935b711775b4d6655d06289dca))
* **web:** use close-trade basis for daily realized pnl table ([56bc2c8](https://github.com/bmf-san/gogocoin/commit/56bc2c8c4af6157cd17e180c24197c2669e60791))
* **web:** use close-trade basis for daily realized pnl table ([45ea75b](https://github.com/bmf-san/gogocoin/commit/45ea75b4bb7960dace887088733604d11178b865))


### Build System

* **deps:** bump github.com/oapi-codegen/runtime from 1.4.0 to 1.4.1 ([#109](https://github.com/bmf-san/gogocoin/issues/109)) ([c67ee3c](https://github.com/bmf-san/gogocoin/commit/c67ee3c4f4f5efc03f0d61fd4da69ef6aa0afc91))


### Miscellaneous

* **lint:** relax gocritic for pkg/backtest import ([d6c5e83](https://github.com/bmf-san/gogocoin/commit/d6c5e83df1dd80588303646d72ac7b464272baf0))
* **main:** release 1.6.1 ([#106](https://github.com/bmf-san/gogocoin/issues/106)) ([a340e4a](https://github.com/bmf-san/gogocoin/commit/a340e4acb003087cfb43e6f64507a31346a63c77))
* **main:** release 1.7.0 ([#111](https://github.com/bmf-san/gogocoin/issues/111)) ([4f9b2ec](https://github.com/bmf-san/gogocoin/commit/4f9b2ec1965ef4fe3df159c94175055ad61ad3fe))

## [1.7.0](https://github.com/bmf-san/gogocoin/compare/v1.6.1...v1.7.0) (2026-05-28)


### Features

* **config:** add trading.observe_symbols for data-collection-only subscriptions ([#107](https://github.com/bmf-san/gogocoin/issues/107)) ([97ee6bd](https://github.com/bmf-san/gogocoin/commit/97ee6bd05f8e2b5a8d957cdab1645f146e7b2da5))
* **web:** align dashboard with exchange-style pnl and positions ([c399371](https://github.com/bmf-san/gogocoin/commit/c39937107b4948702aba48a5a92f23f76f33a440))
* **web:** align dashboard with exchange-style pnl and positions ([#112](https://github.com/bmf-san/gogocoin/issues/112)) ([e7687be](https://github.com/bmf-san/gogocoin/commit/e7687bebfa7c52f8ee774734e98de5619ef242d4))
* **worker:** bar aggregation for strategy worker ([#110](https://github.com/bmf-san/gogocoin/issues/110)) ([d6f7020](https://github.com/bmf-san/gogocoin/commit/d6f70208e7904ed295e2269029fd8c0438c0f85b))


### Bug Fixes

* **example/scalping:** evaluate cooldown/daily-limit against MarketData.Timestamp ([#105](https://github.com/bmf-san/gogocoin/issues/105)) ([4db8add](https://github.com/bmf-san/gogocoin/commit/4db8add99a776e9072801d713ea25a3e7b5d8b67))
* **pnl:** correct dashboard PnL aggregation and retention semantics ([#100](https://github.com/bmf-san/gogocoin/issues/100)) ([33cffb0](https://github.com/bmf-san/gogocoin/commit/33cffb0a61e77d1ec6c8a2c6528cb0ff83917467))


### Documentation

* align bundled scalping strategy docs with actual minimal implementation ([#102](https://github.com/bmf-san/gogocoin/issues/102)) ([abb09e1](https://github.com/bmf-san/gogocoin/commit/abb09e1bbb9bafcac922849e1b40e7ca61b7bc56))


### Build System

* **deps:** bump github.com/mattn/go-sqlite3 from 1.14.42 to 1.14.44 ([#104](https://github.com/bmf-san/gogocoin/issues/104)) ([187eeeb](https://github.com/bmf-san/gogocoin/commit/187eeeb3cd91215f160adf5bf7ce41f50c84c189))
* **deps:** bump github.com/oapi-codegen/runtime from 1.3.1 to 1.4.0 ([#90](https://github.com/bmf-san/gogocoin/issues/90)) ([56124fe](https://github.com/bmf-san/gogocoin/commit/56124feca6f23f69df312613115d9e4af03920a8))


### Continuous Integration

* **dependabot:** add example/ ecosystem and use Conventional Commits prefixes ([#96](https://github.com/bmf-san/gogocoin/issues/96)) ([9810bb0](https://github.com/bmf-san/gogocoin/commit/9810bb0f7ae42ffea84a45b71cae88ef7709c824))
* **deps:** bump actions/labeler from 5 to 6 ([#99](https://github.com/bmf-san/gogocoin/issues/99)) ([1c9f113](https://github.com/bmf-san/gogocoin/commit/1c9f1137073506f0db27ae31fa9a8d3d728b188a))
* **deps:** bump amannn/action-semantic-pull-request from 5 to 6 ([#98](https://github.com/bmf-san/gogocoin/issues/98)) ([d1fd61b](https://github.com/bmf-san/gogocoin/commit/d1fd61bfb5feb312b1a10ff925622587a24930d0))
* **deps:** bump googleapis/release-please-action from 4 to 5 ([#103](https://github.com/bmf-san/gogocoin/issues/103)) ([0989ba6](https://github.com/bmf-san/gogocoin/commit/0989ba62c007d729fc90a771e47eb24e95504e59))
* **deps:** bump kentaro-m/auto-assign-action from 2.0.0 to 2.0.2 ([#97](https://github.com/bmf-san/gogocoin/issues/97)) ([25521da](https://github.com/bmf-san/gogocoin/commit/25521da178cde17bae6a6108a94d197a56121eb5))


### Miscellaneous

* **config:** bump example retention_days from 1 to 90 ([#101](https://github.com/bmf-san/gogocoin/issues/101)) ([b2b1a3c](https://github.com/bmf-san/gogocoin/commit/b2b1a3c1efd2c99f33b4e41b01a839547f57df17))
* **main:** release 1.6.1 ([#106](https://github.com/bmf-san/gogocoin/issues/106)) ([a340e4a](https://github.com/bmf-san/gogocoin/commit/a340e4acb003087cfb43e6f64507a31346a63c77))

## [1.6.1](https://github.com/bmf-san/gogocoin/compare/v1.6.0...v1.6.1) (2026-05-11)


### Bug Fixes

* **example/scalping:** evaluate cooldown/daily-limit against MarketData.Timestamp ([#105](https://github.com/bmf-san/gogocoin/issues/105)) ([4db8add](https://github.com/bmf-san/gogocoin/commit/4db8add99a776e9072801d713ea25a3e7b5d8b67))
* **pnl:** correct dashboard PnL aggregation and retention semantics ([#100](https://github.com/bmf-san/gogocoin/issues/100)) ([33cffb0](https://github.com/bmf-san/gogocoin/commit/33cffb0a61e77d1ec6c8a2c6528cb0ff83917467))
* remove Japanese content accidentally appended to English DESIGN_DOC.md ([#87](https://github.com/bmf-san/gogocoin/issues/87)) ([e7b8965](https://github.com/bmf-san/gogocoin/commit/e7b8965c55f96bad0d992ddc29e94e24387f4a08))


### Refactors

* **ci:** workflow の重複コマンドを Makefile ターゲットに置換 ([#94](https://github.com/bmf-san/gogocoin/issues/94)) ([3325c3c](https://github.com/bmf-san/gogocoin/commit/3325c3ce558e6f43a99abf480bc4f6abb44abe67))


### Documentation

* align bundled scalping strategy docs with actual minimal implementation ([#102](https://github.com/bmf-san/gogocoin/issues/102)) ([abb09e1](https://github.com/bmf-san/gogocoin/commit/abb09e1bbb9bafcac922849e1b40e7ca61b7bc56))


### Tests

* pkg/strategy と internal/utils のユニットテストを追加 ([#95](https://github.com/bmf-san/gogocoin/issues/95)) ([c7cde2e](https://github.com/bmf-san/gogocoin/commit/c7cde2e93d28a7ac544247e1186091d9a20836c2))


### Build System

* **deps:** bump github.com/mattn/go-sqlite3 from 1.14.37 to 1.14.42 ([#89](https://github.com/bmf-san/gogocoin/issues/89)) ([859c7f3](https://github.com/bmf-san/gogocoin/commit/859c7f399c7904daa1c64dd4522bcb9dff5c1a57))
* **deps:** bump github.com/mattn/go-sqlite3 from 1.14.42 to 1.14.44 ([#104](https://github.com/bmf-san/gogocoin/issues/104)) ([187eeeb](https://github.com/bmf-san/gogocoin/commit/187eeeb3cd91215f160adf5bf7ce41f50c84c189))
* **deps:** bump github.com/oapi-codegen/runtime from 1.3.1 to 1.4.0 ([#90](https://github.com/bmf-san/gogocoin/issues/90)) ([56124fe](https://github.com/bmf-san/gogocoin/commit/56124feca6f23f69df312613115d9e4af03920a8))


### Continuous Integration

* add PR automation workflows (auto-assign, semantic title, labeler) ([#91](https://github.com/bmf-san/gogocoin/issues/91)) ([909b55f](https://github.com/bmf-san/gogocoin/commit/909b55fb8c06251596653877896a75847de339e9))
* **dependabot:** add example/ ecosystem and use Conventional Commits prefixes ([#96](https://github.com/bmf-san/gogocoin/issues/96)) ([9810bb0](https://github.com/bmf-san/gogocoin/commit/9810bb0f7ae42ffea84a45b71cae88ef7709c824))
* **deps:** bump actions/labeler from 5 to 6 ([#99](https://github.com/bmf-san/gogocoin/issues/99)) ([1c9f113](https://github.com/bmf-san/gogocoin/commit/1c9f1137073506f0db27ae31fa9a8d3d728b188a))
* **deps:** bump amannn/action-semantic-pull-request from 5 to 6 ([#98](https://github.com/bmf-san/gogocoin/issues/98)) ([d1fd61b](https://github.com/bmf-san/gogocoin/commit/d1fd61bfb5feb312b1a10ff925622587a24930d0))
* **deps:** bump googleapis/release-please-action from 4 to 5 ([#103](https://github.com/bmf-san/gogocoin/issues/103)) ([0989ba6](https://github.com/bmf-san/gogocoin/commit/0989ba62c007d729fc90a771e47eb24e95504e59))
* **deps:** bump kentaro-m/auto-assign-action from 2.0.0 to 2.0.2 ([#97](https://github.com/bmf-san/gogocoin/issues/97)) ([25521da](https://github.com/bmf-san/gogocoin/commit/25521da178cde17bae6a6108a94d197a56121eb5))
* harden CI pipeline and add security workflows (govulncheck, CodeQL, SECURITY.md) ([#93](https://github.com/bmf-san/gogocoin/issues/93)) ([4a29344](https://github.com/bmf-san/gogocoin/commit/4a293447e21cd0d231f0c5bd66e1d3f08fb0b700))
* introduce release-please for CHANGELOG.md and automated releases ([#92](https://github.com/bmf-san/gogocoin/issues/92)) ([9a9c014](https://github.com/bmf-san/gogocoin/commit/9a9c01491ba80ddc1f9207d767a12dcea64666d2))


### Miscellaneous

* **config:** bump example retention_days from 1 to 90 ([#101](https://github.com/bmf-san/gogocoin/issues/101)) ([b2b1a3c](https://github.com/bmf-san/gogocoin/commit/b2b1a3c1efd2c99f33b4e41b01a839547f57df17))
