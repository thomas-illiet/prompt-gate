# Release Process

Backend releases are created by GitHub Actions when a semver tag matching
`vX.Y.Z` is pushed to the repository.

The release workflow validates the backend, validates the frontend, builds
standalone release archives, builds the Docker image, pushes it to GitHub
Container Registry, and creates a GitHub Release with downloadable assets.

## Prerequisites

- Write access to `thomas-illiet/prompt-gate`.
- GitHub Packages enabled for the repository or organization.
- Workflow token permissions for package publishing and release creation.
- A clean local validation run before tagging.

## Pre-Release Verification

From `main`:

```sh
git checkout main
git pull --ff-only origin main
make fmt-check
make vet
make test
make build
docker compose config --quiet
docker build -t prompt-gate-backend:test .
```

If the frontend changed, also run:

```sh
cd frontend
npm ci
npm run lint:check
npm run typecheck
npm test -- --run
npm run generate
```

## Create A Release Tag

Choose a semver version without the `v` prefix, then create and push an
annotated tag with the prefix.

```sh
export VERSION=0.1.0
git tag -a "v${VERSION}" -m "Release v${VERSION}"
git push origin "v${VERSION}"
```

The tag push starts the `Release` workflow.

## Pipeline Behavior

The release pipeline:

- checks Go formatting
- runs `go vet`
- runs backend tests
- installs frontend dependencies
- runs frontend lint, typecheck, and tests
- generates frontend static assets
- validates the Compose file
- builds the backend binary
- cross-builds standalone release archives for macOS ARM64, Linux AMD64, and
  Windows AMD64
- verifies each archive contains the expected binary and `public/index.html`
- builds and pushes the Docker image
- creates a GitHub Release with generated notes and release archives

## Published Release Assets

For Git tag `v0.1.0`, the workflow attaches:

```text
promptgate_0.1.0_darwin_arm64.tar.gz
promptgate_0.1.0_linux_amd64.tar.gz
promptgate_0.1.0_windows_amd64.zip
```

Each archive contains:

- `promptgate` or `promptgate.exe`
- `public/` with the generated Nuxt static frontend
- `README.txt` with a minimal standalone launch example

To run the Linux archive with the bundled frontend assets:

```sh
tar -xzf promptgate_0.1.0_linux_amd64.tar.gz
cd promptgate_0.1.0_linux_amd64
PROMPTGATE_STATIC_ASSETS_DIR="$(pwd)/public" ./promptgate api
```

The standalone binary still requires PostgreSQL, Redis, and OIDC/Keycloak
configuration. See [Environment variables](environment.md) for required
runtime settings.

## Published Docker Tags

For Git tag `v0.1.0`, the workflow publishes:

```text
ghcr.io/thomas-illiet/prompt-gate:v0.1.0
ghcr.io/thomas-illiet/prompt-gate:0.1.0
ghcr.io/thomas-illiet/prompt-gate:latest
ghcr.io/thomas-illiet/prompt-gate:sha-<short-sha>
```

Use the immutable `vX.Y.Z` tag for production deployments.

## Verify A Release

After the workflow completes:

```sh
docker pull ghcr.io/thomas-illiet/prompt-gate:v0.1.0
docker run --rm --entrypoint id ghcr.io/thomas-illiet/prompt-gate:v0.1.0
```

The container should run as the non-root `promptgate` user with UID `10001`.

Check the GitHub Releases page for generated release notes and verify that the
image digest matches the deployed artifact.

Also verify that the release assets are attached:

```sh
gh release view v0.1.0 --json assets --jq '.assets[].name'
```

## Rollback

Redeploy the previous stable versioned image tag:

```sh
docker pull ghcr.io/thomas-illiet/prompt-gate:v0.0.9
```

Avoid using `latest` for production rollbacks because it moves every time a new
release tag is published.

Related docs:

- [Deployment](deployment.md)
- [Environment variables](environment.md)
- [Development guide](development.md)
