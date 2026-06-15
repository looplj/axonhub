// Package racetest holds opt-in, build-tagged integration tests that need a real
// concurrent database (PostgreSQL). It lives in its own package because it
// imports both internal/server/biz and internal/server/db, and db transitively
// imports biz (via datamigrate) — so the test cannot live inside the biz package
// itself without creating an import cycle.
//
// Run it with:
//
//	AXONHUB_TEST_PG_DSN="postgres://postgres:postgres@localhost:55432/axonhub?sslmode=disable" \
//	  go test ./internal/server/biz/racetest/ -tags pgrace -run TestAPIKeyNameRace_Postgres -v -count=1
package racetest
