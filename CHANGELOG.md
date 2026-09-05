# Changelog

All notable changes to StatusFlow Go are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-09-04

### Added

- Initial public release of the `github.com/aprog93/statusflow-go` Go module.
- Consistent JSON response envelopes for successful and error responses.
- `HTTPError` and `ApplicationError` types with HTTP status handling and
  validated application error codes.
- English and Spanish catalog messages with language-aware constructors and
  adapters.
- `net/http` integration through `WriteJSON`, `WriteError`, and `Adapt`.
- GitHub Actions CI for formatting, vetting, tests, and race detection on Go
  1.22.

### Changed

- Established the pre-1.0 public API and documented its response, error, and
  `net/http` integration contracts.
- Documented the standard-library-only runtime dependency and release guidance.

### Security

- Unknown errors are converted to a generic internal-server-error response so
  private causes are not serialized.
- Internal causes remain available to trusted server-side code through
  `errors.Is` and `errors.As`, while public details remain caller-controlled.
- Application error codes are restricted to safe lowercase ASCII identifiers.

### Testing

- Added coverage for constructors, status validation, JSON serialization,
  language fallbacks, error redaction, `net/http` integration, and end-to-end
  requests through `httptest.NewServer`.

[0.1.0]: https://github.com/aprog93/statusflow-go/releases/tag/v0.1.0
