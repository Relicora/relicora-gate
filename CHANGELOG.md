# Changelog

All notable changes to this project will be documented in this file.

## [v0.4.0] - 2026-08-01
### Added
- parameterized routing with `:param` and `*path`
- `Router.HandleFunc` for router-level handler functions
- `Router.Group` alias for nested route groups
- `RouteParams(r *http.Request)` helper for extracted route parameters
- custom `404` and `405` handlers via `App.NotFoundHandler` and `App.MethodNotAllowedHandler`

### Changed
- centralized route matching in `routeManager` for consistent method/resolution behavior
- `App.Handle` and `Router.Handle` now support normalized route registration with prefixes

## [v0.3.2] - 2026-08-01
### Added
- `App.Handle` and `App.HandleFunc` for registering handlers with multiple HTTP methods
- Router `Handle` for grouped method registration

### Fixed
- normalized route paths and router prefixes for consistent route registration
- normalized HTTP methods to uppercase during route registration

## [v0.3.1] - 2026-08-01
### Fixed
- Allow registering the same route path with different HTTP methods
- Return correct `Allow` header for method-mismatched requests

## [v0.3.0] - 2026-07-14
### Added
- New `App` method `Shutdown`: perform graceful server shutdown

### Fixed
- Method `ListenAndServe` now return error 

## [v0.2.0] - 2026-04-25
### Added
- New `middleware` package with request logger, panic recovery, and timeout middlewares.
- Request logger now records response duration in log output.

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
