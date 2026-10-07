# Glasswaves migration fork

This fork retains `github.com/golang-migrate/migrate/v4` as its module path.
The maintained branch is `glasswaves/dsql-postgres-compat`; upstream `master`
remains available for comparison. Consumers pin a commit with a Go `replace`
directive pointing to `github.com/glasswavesco/migrate/v4`.

It includes upstream PR #1449 at
`6518c5aa2d00aedccd6a20b3cb33151c1a4c57f7`, the continuation of Joe's original
DSQL driver PR #1430. The DSQL driver uses the AWS IAM connector, durable row
locking, DSQL version bookkeeping, and waits for asynchronous jobs.

The pgx/v5 driver adds opt-in `DSQLCompatibility` / `x-dsql-compatibility=true`.
It translates the ASYNC keyword in CREATE [UNIQUE] INDEX and ALTER TABLE
statements so one ordered migration chain can run on disposable PostgreSQL.
It preserves literals, comments and quoted identifiers. See
`database/pgx/v5/README.md` for the supported scope and limitations.

The fork also fixes validity checks for skipped async indexes whose target table
is in a different schema from migration metadata. Explicit table qualifiers are
used to locate the index, and an unresolved index fails rather than being marked
clean. A live regression test covers valid and failed unique indexes across schemas.

Checks:

```
go test -race ./database/dsql ./database/pgx/v5
go test -tags 'dsql pgx5' ./internal/cli ./cmd/migrate
go vet ./database/dsql ./database/pgx/v5
```

Live DSQL tests additionally require `DSQL_CLUSTER_ENDPOINT`, `DSQL_TEST_SCHEMA`
and sandbox IAM credentials. Use a dedicated disposable cluster and schema;
the suite exercises Drop and must never target application or production data.
