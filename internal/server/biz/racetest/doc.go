// Package racetest holds opt-in, build-tagged integration tests that need a real
// concurrent database (PostgreSQL, MySQL, or TiDB). It lives in its own package
// because it imports both internal/server/biz and internal/server/db, and db
// transitively imports biz (via datamigrate) — so the test cannot live inside the
// biz package itself without creating an import cycle.
//
// The test is gated behind the `dbrace` build tag and runs one subtest per dialect
// whose DSN env var is set (an unset dialect is skipped). SQLite needs no server —
// point it at a temp file:
//
//	AXONHUB_TEST_PG_DSN="postgres://postgres:postgres@localhost:55432/axonhub?sslmode=disable" \
//	AXONHUB_TEST_MYSQL_DSN="root:root@tcp(localhost:3306)/axonhub?parseTime=true" \
//	AXONHUB_TEST_TIDB_DSN="root@tcp(localhost:4000)/axonhub?parseTime=true" \
//	AXONHUB_TEST_SQLITE_DSN="file:/tmp/race.db" \
//	  go test ./internal/server/biz/racetest/ -tags dbrace -run TestAPIKeyNameRace -v -count=1
//
// For a high-latency TiDB (e.g. TiDB Cloud across regions) raise the lock wait so
// the serialized burst does not trip lock-wait-timeout: append
// &innodb_lock_wait_timeout=600 to the TiDB DSN.
package racetest
