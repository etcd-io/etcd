# Release Process

This document describes how to create a new release of Unqueryvet and its VS Code extension.

## Prerequisites

- Push access to the repository
- Go 1.21+ installed
- Node.js 20+ installed
- Git configured

## Release Workflow

### Option 1: Fully Automated (Recommended)

1. **Create a Git tag:**
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. **Create GitHub Release:**
   - Go to https://github.com/MirrexOne/unqueryvet/releases/new
   - Select the tag you just pushed (v1.0.0)
   - Title: `v1.0.0`
   - Description: List of changes
   - Click "Publish release"

3. **What happens automatically:**
   - ✅ GitHub Actions builds LSP binaries for all platforms
   - ✅ Binaries are attached to the release
   - ✅ Checksums file is generated
   - ✅ VS Code extension is built
   - ✅ VS Code extension is published to Marketplace
   - ✅ Extension .vsix is attached to the release

4. **Wait 5-10 minutes** for workflows to complete

5. **Verify:**
   - Check release has all LSP binaries attached
   - Check VS Code Marketplace: https://marketplace.visualstudio.com/items?itemName=mirrexdev.unqueryvet
   - Test automatic LSP download in a fresh VS Code instance

### Option 2: Manual Build

If you need to build locally:

1. **Build LSP binaries:**
   ```bash
   # Using Task
   task build:lsp:release

   # Or using script directly
   ./scripts/build-lsp.sh v1.0.0   # Linux/macOS
   # OR
   powershell ./scripts/build-lsp.ps1 -Version v1.0.0  # Windows
   ```

2. **Check dist/ folder:**
   ```bash
   ls -lh dist/
   ```

   You should see:
   - unqueryvet-lsp-windows-amd64.exe
   - unqueryvet-lsp-windows-arm64.exe
   - unqueryvet-lsp-linux-amd64
   - unqueryvet-lsp-linux-arm64
   - unqueryvet-lsp-darwin-amd64
   - unqueryvet-lsp-darwin-arm64
   - checksums.txt

3. **Upload to GitHub Release manually:**
   - Create release on GitHub
   - Upload all files from dist/

4. **Build VS Code extension:**
   ```bash
   cd extensions/vscode
   npm install
   npm run compile
   npx @vscode/vsce package
   ```

5. **Publish VS Code extension:**
   ```bash
   npx @vscode/vsce publish -p YOUR_PAT_TOKEN
   ```

## Platform Support

LSP binaries are built for:

| Platform | Architecture | File |
|----------|-------------|------|
| Windows  | amd64       | unqueryvet-lsp-windows-amd64.exe |
| Windows  | arm64       | unqueryvet-lsp-windows-arm64.exe |
| Linux    | amd64       | unqueryvet-lsp-linux-amd64 |
| Linux    | arm64       | unqueryvet-lsp-linux-arm64 |
| macOS    | amd64       | unqueryvet-lsp-darwin-amd64 |
| macOS    | arm64 (M1+) | unqueryvet-lsp-darwin-arm64 |

## Version Numbering

Follow Semantic Versioning (semver):

- **Major** (v2.0.0): Breaking changes
- **Minor** (v1.1.0): New features, backward compatible
- **Patch** (v1.0.1): Bug fixes, backward compatible

## Checklist Before Release

- [ ] All tests passing (`task test`)
- [ ] Code formatted (`task fmt`)
- [ ] Linter passing (`task lint`)
- [ ] CHANGELOG.md updated
- [ ] Version bumped in:
  - [ ] extensions/vscode/package.json
  - [ ] extensions/vscode/CHANGELOG.md
- [ ] README.md up to date
- [ ] All PRs merged
- [ ] No breaking changes (or documented)

## Post-Release

1. **Test installation:**
   ```bash
   # Test Go installation
   go install github.com/MirrexOne/unqueryvet/cmd/unqueryvet@latest

   # Test LSP installation
   go install github.com/MirrexOne/unqueryvet/cmd/unqueryvet-lsp@latest
   ```

2. **Test VS Code extension:**
   - Install from Marketplace
   - Open a Go file
   - Verify automatic LSP download works
   - Verify diagnostics appear

3. **Update documentation** if needed

4. **Announce release:**
   - Twitter/X
   - Reddit r/golang
   - LinkedIn
   - Discord communities

## Troubleshooting

### GitHub Actions fails

- Check workflow logs: https://github.com/MirrexOne/unqueryvet/actions
- Common issues:
  - VSCE_PAT expired → regenerate in Azure DevOps
  - Build error → test locally first
  - Permission error → check GITHUB_TOKEN permissions

### VS Code extension not publishing

- Verify VSCE_PAT secret is set in GitHub
- Check publisher ID matches in package.json (mirrexdev)
- Ensure version number is incremented

### LSP binaries missing

- Check GitHub Actions workflow completed
- Verify tag was pushed correctly
- Check release was published (not draft)

## Emergency Rollback

If a release has critical bugs:

1. **Mark as pre-release** on GitHub
2. **Unpublish VS Code extension:**
   ```bash
   npx @vscode/vsce unpublish mirrexdev.unqueryvet@VERSION
   ```
3. **Fix issues** and create new patch release
4. **Communicate** to users about the issue

## Files Modified Per Release

- `extensions/vscode/package.json` - bump version
- `extensions/vscode/CHANGELOG.md` - add release notes
- `CHANGELOG.md` - add release notes (if exists)
- Git tag - create new tag

## Automation Scripts

| Script | Purpose |
|--------|---------|
| `scripts/build-lsp.sh` | Build LSP for all platforms (Unix) |
| `scripts/build-lsp.ps1` | Build LSP for all platforms (Windows) |
| `.github/workflows/release-lsp.yml` | Auto-build LSP on release |
| `.github/workflows/publish-vscode-extension.yml` | Auto-publish VS Code extension |

## Support

For issues with releases:
- GitHub Issues: https://github.com/MirrexOne/unqueryvet/issues
- GitHub Discussions: https://github.com/MirrexOne/unqueryvet/discussions
