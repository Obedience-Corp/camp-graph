# Releasing `camp-graph`

This repo now follows the reusable release pattern for standalone `camp-*`
plugins.

## What one tag publishes

- GitHub release archives for macOS and Linux
- Homebrew cask in `Obedience-Corp/homebrew-tap`
- AUR package `camp-graph-bin`
- Release checksums and generated shell completions

## One-time setup

1. Add repository secret `HOMEBREW_TAP_GITHUB_TOKEN` with write access to `Obedience-Corp/homebrew-tap`.
2. Create the AUR package repo `camp-graph-bin`, add the release public key there, and store the private key in repository secret `AUR_SSH_KEY`.
3. Confirm `Obedience-Corp/homebrew-tap` exists with a `Casks/` directory.

The release workflow fails early if either secret is missing.

## Release steps

```bash
just release stable
```

GitHub Actions then:

1. reruns lint and unit tests
2. builds release archives with GoReleaser
3. creates the GitHub release
4. updates `Obedience-Corp/homebrew-tap`
5. updates the AUR package repo

If you need an explicit version instead of the next computed stable tag:

```bash
just release tag v0.1.0
```

## Reuse for future `camp-*` plugins

Copy this file set into the new plugin repo:

- `.github/workflows/test.yaml`
- `.github/workflows/release.yaml`
- `.goreleaser.yaml`
- `.justfiles/release.just`
- `scripts/completions.sh`
- `docs/releasing.md`

Then make only these repo-specific changes:

1. Rename the module path and binary path to `camp-<name>`.
2. Update the README description and install examples.
3. Update the Homebrew/AUR metadata strings in `.goreleaser.yaml`.
4. Ensure the AUR repo name is `camp-<name>-bin`.

That keeps plugin distribution mechanics consistent across the rest of the
`camp-*` plugin ecosystem.
