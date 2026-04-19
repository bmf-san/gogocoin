# Changelog

## [1.6.1](https://github.com/bmf-san/gogocoin/compare/v1.6.0...v1.6.1) (2026-04-19)


### Bug Fixes

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
* **deps:** bump github.com/oapi-codegen/runtime from 1.3.1 to 1.4.0 ([#90](https://github.com/bmf-san/gogocoin/issues/90)) ([56124fe](https://github.com/bmf-san/gogocoin/commit/56124feca6f23f69df312613115d9e4af03920a8))


### Continuous Integration

* add PR automation workflows (auto-assign, semantic title, labeler) ([#91](https://github.com/bmf-san/gogocoin/issues/91)) ([909b55f](https://github.com/bmf-san/gogocoin/commit/909b55fb8c06251596653877896a75847de339e9))
* **dependabot:** add example/ ecosystem and use Conventional Commits prefixes ([#96](https://github.com/bmf-san/gogocoin/issues/96)) ([9810bb0](https://github.com/bmf-san/gogocoin/commit/9810bb0f7ae42ffea84a45b71cae88ef7709c824))
* **deps:** bump actions/labeler from 5 to 6 ([#99](https://github.com/bmf-san/gogocoin/issues/99)) ([1c9f113](https://github.com/bmf-san/gogocoin/commit/1c9f1137073506f0db27ae31fa9a8d3d728b188a))
* **deps:** bump amannn/action-semantic-pull-request from 5 to 6 ([#98](https://github.com/bmf-san/gogocoin/issues/98)) ([d1fd61b](https://github.com/bmf-san/gogocoin/commit/d1fd61bfb5feb312b1a10ff925622587a24930d0))
* **deps:** bump kentaro-m/auto-assign-action from 2.0.0 to 2.0.2 ([#97](https://github.com/bmf-san/gogocoin/issues/97)) ([25521da](https://github.com/bmf-san/gogocoin/commit/25521da178cde17bae6a6108a94d197a56121eb5))
* harden CI pipeline and add security workflows (govulncheck, CodeQL, SECURITY.md) ([#93](https://github.com/bmf-san/gogocoin/issues/93)) ([4a29344](https://github.com/bmf-san/gogocoin/commit/4a293447e21cd0d231f0c5bd66e1d3f08fb0b700))
* introduce release-please for CHANGELOG.md and automated releases ([#92](https://github.com/bmf-san/gogocoin/issues/92)) ([9a9c014](https://github.com/bmf-san/gogocoin/commit/9a9c01491ba80ddc1f9207d767a12dcea64666d2))


### Miscellaneous

* **config:** bump example retention_days from 1 to 90 ([#101](https://github.com/bmf-san/gogocoin/issues/101)) ([b2b1a3c](https://github.com/bmf-san/gogocoin/commit/b2b1a3c1efd2c99f33b4e41b01a839547f57df17))
