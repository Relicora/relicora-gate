# Changelog

Все заметные изменения в проекте `relicora-gate` документируются в этом файле.

## [Unreleased]
- 

## [v0.1.0] - 2026-04-20
### Added
- Initial release of `relicora-gate`
- `App` constructor refactored to use functional options:
  - `WithAddr`
  - `WithPort`
  - `WithLogger`
- HTTP method-specific route helpers:
  - `Get`
  - `Post`
  - `Put`
  - `Delete`
- Middleware support for `App` and nested `Router`
- Nested router creation with `App.NewRouter` and `Router.NewRouter`

### Fixed
- HTTP routing and middleware application in request handling

### Notes
- Default server address is `:8080`
- `ListenAndServe` logs startup information before serving
