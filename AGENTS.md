<!-- crag:auto-start -->
# AGENTS.md

> Generated from governance.md by crag. Regenerate: `crag compile --target agents-md`

## Project: etcd


## Quality Gates

All changes must pass these checks before commit:

### Lint
1. `go vet ./...`

### Test
1. `go test ./...`
2. `make test`
3. `make verify`

### Build
1. `make build`

### Ci (inferred from workflow)
1. `make antithesis-build-client-docker-image`
2. `docker compose -f config/docker-compose-${{ matrix.node-count }}-node.yml logs`

### Contributor docs (advisory — confirm before enforcing)
1. `make verify-*  # from CONTRIBUTING.md`
2. `make verify-bom  # from CONTRIBUTING.md`
3. `make test-unit  # from CONTRIBUTING.md`

## Coding Standards

- Stack: go, docker
- Follow project commit conventions

## Architecture

- Type: microservices (cargo)
- Services: api, pkg, v3, server

## Key Directories

- `.github/` — CI/CD
- `api/` — backend
- `client/` — frontend
- `pkg/` — source
- `scripts/` — tooling
- `server/` — backend
- `tests/` — tests
- `tools/` — tooling

## Testing

- Framework: go test
- Layout: structured + e2e
- Coverage: configured

## Anti-Patterns

Do not:
- Do not ignore returned errors — handle or explicitly discard with `_ =`
- Do not use `panic()` in library code — return errors instead
- Do not use `init()` functions unless absolutely necessary
- Do not use `latest` tag in FROM — pin to a specific version
- Do not run containers as root — use a non-root USER

## Security

- No hardcoded secrets — grep for sk_live, AKIA, password= before commit

## Workflow

1. Read `governance.md` at the start of every session — it is the single source of truth.
2. Run all mandatory quality gates before committing.
3. If a gate fails, fix the issue and re-run only the failed gate.
4. Use the project commit conventions for all changes.

<!-- crag:auto-end -->
