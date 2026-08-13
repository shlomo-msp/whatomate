# Environment Override Contract Design

## Goal

Make fork Docker deployments pass PostgreSQL and Redis credentials to Whatomate
using the double-underscore environment-variable contract required by the
current parser, while keeping existing host-facing Compose variable names.

## Design

`docker/docker-compose.yml` continues to consume `POSTGRES_USER`,
`POSTGRES_PASSWORD`, `POSTGRES_DB`, and `REDIS_PASSWORD`. It maps those values
to `WHATOMATE_DATABASE__*` and `WHATOMATE_REDIS__PASSWORD` for the application.
No parser compatibility alias is added because a single underscore is
ambiguous when section and field names themselves contain underscores.

All environment examples and user documentation describe a double underscore
only between the section and field. `FORK_MAINTENANCE.md` records why the fork
must preserve this wiring and instructs future upstream syncs to replace the
fork patch if upstream later supplies an equivalent fix.

## Deployment Compatibility

Existing hosts using the standard Compose variables do not rename them. A host
only needs a configuration edit if it directly declares legacy
`WHATOMATE_<SECTION>_<KEY>` application overrides or lacks the already-required
`REDIS_PASSWORD` value. Before rollout, operators compare the standard
PostgreSQL and Redis values with the credentials currently used by
`config.toml` and the initialized services, because the corrected application
overrides become authoritative after restart.

## Verification

Validate the environment naming contract with a focused source check, render
the Compose model using the example environment, and run the existing config
package tests that exercise double-underscore parsing.
