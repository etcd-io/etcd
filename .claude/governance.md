# Governance — etcd
# Inferred by crag analyze — review and adjust as needed

## Identity
- Project: etcd
- Stack: go, docker
- Workspace: go

## Gates (run in order, stop on failure)
### Lint
- go vet ./...

### Test
- go test ./...
- make test
- make verify

### Build
- make build

### CI (inferred from workflow)
- make antithesis-build-client-docker-image
- docker compose -f config/docker-compose-${{ matrix.node-count }}-node.yml logs

### Contributor docs (ADVISORY — confirm before enforcing)
- make verify-*  # from CONTRIBUTING.md
- make verify-bom  # from CONTRIBUTING.md
- make test-unit  # from CONTRIBUTING.md

## Advisories (informational, not enforced)
- hadolint Dockerfile  # [ADVISORY]
- actionlint  # [ADVISORY]

## Branch Strategy
- Trunk-based development
- Free-form commits
- Commit trailer: Co-Authored-By: Claude <noreply@anthropic.com>

## Security
- No hardcoded secrets — grep for sk_live, AKIA, password= before commit

## Autonomy
- Auto-commit after gates pass

## Deployment
- Target: docker
- CI: github-actions

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

## Dependencies
- Package manager: go (go.sum)
- Go: >=1.26

## Anti-Patterns

Do not:
- Do not ignore returned errors — handle or explicitly discard with `_ =`
- Do not use `panic()` in library code — return errors instead
- Do not use `init()` functions unless absolutely necessary
- Do not use `latest` tag in FROM — pin to a specific version
- Do not run containers as root — use a non-root USER

