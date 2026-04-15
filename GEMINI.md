<!-- crag:auto-start -->
# GEMINI.md

> Generated from governance.md by crag. Regenerate: `crag compile --target gemini`

## Project Context

- **Name:** etcd
- **Stack:** go, docker
- **Runtimes:** node, go, docker

## Rules

### Quality Gates

Run these checks in order before committing any changes:

1. [lint] `go vet ./...`
2. [test] `go test ./...`
3. [test] `make test`
4. [test] `make verify`
5. [build] `make build`
6. [ci (inferred from workflow)] `make antithesis-build-client-docker-image`
7. [ci (inferred from workflow)] `docker compose -f config/docker-compose-${{ matrix.node-count }}-node.yml logs`
8. [contributor docs (advisory — confirm before enforcing)] `make verify-*  # from CONTRIBUTING.md`
9. [contributor docs (advisory — confirm before enforcing)] `make verify-bom  # from CONTRIBUTING.md`
10. [contributor docs (advisory — confirm before enforcing)] `make test-unit  # from CONTRIBUTING.md`

### Security

- No hardcoded secrets — grep for sk_live, AKIA, password= before commit

### Workflow

- Follow project commit conventions
- Run quality gates before committing
- Review security implications of all changes

<!-- crag:auto-end -->
