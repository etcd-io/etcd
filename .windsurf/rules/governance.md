---
trigger: always_on
description: Governance rules for etcd — compiled from governance.md by crag
---

# Windsurf Rules — etcd

Generated from governance.md by crag. Regenerate: `crag compile --target windsurf`

## Project

(No description)

**Stack:** go, docker

## Runtimes

node, go, docker

## Cascade Behavior

When Windsurf's Cascade agent operates on this project:

- **Always read governance.md first.** It is the single source of truth for quality gates and policies.
- **Run all mandatory gates before proposing changes.** Stop on first failure.
- **Respect classifications.** OPTIONAL gates warn but don't block. ADVISORY gates are informational.
- **Respect path scopes.** Gates with a `path:` annotation must run from that directory.
- **No destructive commands.** Never run rm -rf, dd, DROP TABLE, force-push to main, curl|bash, docker system prune.
- - No hardcoded secrets — grep for sk_live, AKIA, password= before commit
- Follow the project commit conventions.

## Quality Gates (run in order)

1. `go vet ./...`
2. `go test ./...`
3. `make test`
4. `make verify`
5. `make build`
6. `make antithesis-build-client-docker-image`
7. `docker compose -f config/docker-compose-${{ matrix.node-count }}-node.yml logs`
8. `make verify-*  # from CONTRIBUTING.md`
9. `make verify-bom  # from CONTRIBUTING.md`
10. `make test-unit  # from CONTRIBUTING.md`

## Rules of Engagement

1. **Minimal changes.** Don't rewrite files that weren't asked to change.
2. **No new dependencies** without explicit approval.
3. **Prefer editing** existing files over creating new ones.
4. **Always explain** non-obvious changes in commit messages.
5. **Ask before** destructive operations (delete, rename, migrate schema).

---

**Tool:** crag — https://www.npmjs.com/package/@whitehatd/crag
