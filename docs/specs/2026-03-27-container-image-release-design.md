# Container Image Release Pipeline

## Problem

Users must install apko and run `claude-sandbox build` before they can use the tool. This is friction — most users just want to `execute` and have it work.

## Solution

Publish a signed, multi-arch container image to GHCR as part of the release workflow. Users pull the image instead of building locally. The image is signed with keyless Sigstore/Fulcio, includes an SBOM, and has SLSA provenance attestation.

## Image Registry

`ghcr.io/dakaneye/claude-sandbox`

### Tag Strategy

On release `v2.1.0`:
- `ghcr.io/dakaneye/claude-sandbox:v2.1.0` (exact version, immutable)
- `ghcr.io/dakaneye/claude-sandbox:2` (major version, rolling)
- `ghcr.io/dakaneye/claude-sandbox:latest` (rolling)

## Release Workflow

New `container` job in `.github/workflows/release.yml`, runs after GoReleaser:

### Steps

1. **Checkout** with fetch-depth 0
2. **Set up Docker Buildx** for multi-arch builds
3. **Login to GHCR** via `docker/login-action` with `GITHUB_TOKEN`
4. **Install apko** for base image build
5. **Build base image** — `apko build` with embedded apko.yaml, output multi-arch TAR
6. **Build + push final image** — `docker buildx build` with prebake.dockerfile, push to GHCR with all tags
7. **Install cosign**
8. **Sign image** — keyless cosign sign with GitHub Actions OIDC identity
9. **Generate SBOM** — syft scan of the pushed image, output in SPDX JSON format
10. **Attach SBOM** — `cosign attach sbom` to the image
11. **Attest provenance** — `cosign attest` with SLSA provenance predicate

### Permissions

```yaml
permissions:
  contents: write    # GoReleaser releases
  packages: write    # GHCR push
  id-token: write    # Sigstore OIDC keyless signing
```

## CLI Changes

### Image Resolution Order

Update `container.go` to resolve images in this order:

1. Check if `claude-sandbox:latest` exists locally → use it
2. Pull `ghcr.io/dakaneye/claude-sandbox:<version>` → use it
3. Fall back to local build via `claude-sandbox build`

### New Function

```go
// Pull pulls the sandbox image from GHCR.
func Pull(version string) error
```

### DefaultImage Update

Change `DefaultImage` from `claude-sandbox:latest` to `ghcr.io/dakaneye/claude-sandbox:latest` so users get the public image by default. Local builds still work via `claude-sandbox build` which tags as `claude-sandbox:latest`.

### Image Existence Check Update

`ImageExists` should check for both the GHCR tag and the local tag.

## Verification

Users can verify the image:

```bash
# Verify signature
cosign verify ghcr.io/dakaneye/claude-sandbox:latest \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/dakaneye/claude-sandbox'

# Download and view SBOM
cosign download sbom ghcr.io/dakaneye/claude-sandbox:latest | jq .

# Verify SLSA provenance
cosign verify-attestation ghcr.io/dakaneye/claude-sandbox:latest \
  --type slsaprovenance \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/dakaneye/claude-sandbox'
```

## Files Changed

| File | Change |
|------|--------|
| `.github/workflows/release.yml` | Add container build/push/sign/sbom/attest job |
| `internal/container/container.go` | Update `DefaultImage` to GHCR, add image resolution logic |
| `internal/container/build.go` | Add `Pull()` function |
| `internal/container/container_test.go` | Update tests for new image name |
| `README.md` | Add docker pull, verification, and SBOM commands |

## README Updates

### Install section

Add `docker pull` as the primary install method:

```bash
# Pull the container image (recommended)
docker pull ghcr.io/dakaneye/claude-sandbox:latest

# Or build locally (requires apko)
claude-sandbox build
```

### New Verification section

```bash
# Verify image signature
cosign verify ghcr.io/dakaneye/claude-sandbox:latest \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/dakaneye/claude-sandbox'

# View SBOM
cosign download sbom ghcr.io/dakaneye/claude-sandbox:latest | jq .

# Verify SLSA provenance
cosign verify-attestation ghcr.io/dakaneye/claude-sandbox:latest \
  --type slsaprovenance \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/dakaneye/claude-sandbox'
```

## Success Criteria

1. `docker pull ghcr.io/dakaneye/claude-sandbox:latest` works after release
2. Image is multi-arch (amd64 + arm64)
3. `cosign verify` succeeds with GitHub Actions OIDC issuer
4. SBOM is attached and downloadable
5. SLSA provenance attestation is attached and verifiable
6. `claude-sandbox execute` pulls the image automatically if not local
7. `claude-sandbox build` still works as local fallback
