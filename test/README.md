# Test Layout

`test/` is the single entry point for running tests and integration suites.

## Structure

- `test/run/` contains centralized runners:
  - `all.sh` for full test run (`go test ./...`)
  - `race.sh` for race checks
  - `unit.sh` for unit suites (`internal/...` + `proto/...`)
  - `integration.sh` for integration suite (`test/integration`)
- `test/integration/` contains integration scenarios.

## Why unit tests stay near code

In Go, the idiomatic and maintainable approach is colocated unit tests (`*_test.go` in the same package directory). It keeps navigation simple and lowers refactor risk.

Centralized execution is still provided via `test/run/*`.
