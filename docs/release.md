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
- Disposable PostgreSQL 18 and Redis 8 services for integration tests.

## Pre-Release Verification

From `main`:

```sh
git checkout main
git pull --ff-only origin main
make fmt-check
make vet
make staticcheck
PROMPTGATE_REQUIRE_INTEGRATION=1 make coverage
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
bash deploy/helm/prompt-gate/tests/render.sh
bash deploy/examples/opa/test.sh
make build
docker compose config --quiet
docker build -t prompt-gate-backend:test .
```

If the frontend changed, also run:

```sh
cd frontend
npm ci
npm audit --audit-level=low
npm run lint:check
npm run typecheck
npm test -- --run
npm run generate
```

## Create A Release Tag

Choose a semver version without the `v` prefix, then create and push an
annotated tag with the prefix.

```sh
export VERSION=1.4.0
git tag -a "v${VERSION}" -m "Release v${VERSION}"
git push origin "v${VERSION}"
```

The tag push starts the `Release` workflow.

## Pipeline Behavior

Pull requests and tags call the same `validate.yml` workflow. Release publication
depends on successful validation. Required integration services fail the run when
unavailable; they cannot silently turn into skipped tests in CI.

The release pipeline:

- checks Go formatting
- runs `go vet`
- runs the pinned Staticcheck tool
- enforces 70% aggregate Go statement coverage and scans Go/npm vulnerabilities
- validates Helm rendering and the OPA policy and API boundary
- runs backend tests with Redis, including PostgreSQL-specific migration, bootstrap
  concurrency, and quota-reset coverage against PostgreSQL 18
- installs frontend dependencies
- runs frontend lint, typecheck, and tests
- generates frontend static assets
- validates the Compose file
- builds the backend binary
- cross-builds standalone release archives for macOS ARM64, Linux AMD64, and
  Windows AMD64
- verifies each archive contains the expected binary and `public/index.html`
- packages a Helm chart with appVersion equal to the release tag
- creates SHA256SUMS for every archive and the chart
- builds and pushes the Docker image with SBOM and provenance attestations
- creates a GitHub Release with generated notes and release archives

## Published Release Assets

For Git tag `v0.1.0`, the workflow attaches:

```text
promptgate_0.1.0_darwin_arm64.tar.gz
promptgate_0.1.0_linux_amd64.tar.gz
promptgate_0.1.0_windows_amd64.zip
prompt-gate-0.1.0.tgz
SHA256SUMS
```

Each archive contains:

- `promptgate` or `promptgate.exe`
- `public/` with the generated Nuxt static frontend
- `promptgate.example.yaml` with the complete configuration schema
- `README.txt` with a minimal standalone launch example

To run the Linux archive with the bundled frontend assets:

```sh
tar -xzf promptgate_0.1.0_linux_amd64.tar.gz
cd promptgate_0.1.0_linux_amd64
cp promptgate.example.yaml promptgate.yaml
# Fill required values. For the bundled same-origin frontend, set:
# - api.static_assets_dir: public
# - common.public_urls.frontend: http://localhost:8080
# - common.cors.allowed_origins: [http://localhost:8080]
./promptgate --config promptgate.yaml api
```

The standalone binary still requires PostgreSQL, Redis, and OIDC/Keycloak
configuration. See [YAML configuration](environment.md) for required
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
- [YAML configuration](environment.md)
- [Development guide](development.md)

## Local dependency patches

`frontend/vendor/deepmerge` preserves deepmerge 4.3.1's API and MIT license while
rejecting prototype-sensitive keys in both operands (GHSA-gcqx-w48m-w2h4).
There is no upstream fixed release at the time of this change. Its local override
and regression tests must remain until a fixed upstream release replaces it.
`frontend/vendor/minimatch-compat` adapts the import-x lint rule's `.test()` use
to the patched minimatch 11 matcher without reconstructing vulnerable regexes.
Both are installed from versioned local sources by `npm ci`.
