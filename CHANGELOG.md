# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]
- 

## [v0.1.3] - 2026-04-25
### Fixed
- Corrected `WithAddr` + `WithPort` handling so the server address is resolved as `host:port` when a host is provided.

## [v0.1.2] - 2026-04-25
### Added
- GoDoc comments for all public API methods and user-facing package types
- public documentation for `App`, `Router`, route registration, and middleware methods

### Changed
- improved package documentation and developer ergonomics

## [v0.1.1] - 2026-04-25
### Added
- English `README.md` documentation
- `.gitignore` entry for `coverage.out`
- full unit test coverage for `App`, `Router`, middleware, and route methods

### Fixed
- corrected router route registration for `Router.Get`, `Router.Post`, `Router.Put`, and `Router.Delete`

### Changed
- updated documentation and repository metadata

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
