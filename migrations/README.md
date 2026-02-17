# Deprecated Path

This directory is deprecated and is kept only for backward compatibility in the repository layout.

Use canonical SQL migrations from:

`internal/storage/postgres/sql/migrations`

Apply/rollback/status via:

`go run ./cmd/migrate -direction up|down|status`
