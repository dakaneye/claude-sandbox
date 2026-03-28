# Container Image Release Pipeline — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish a signed, multi-arch container image to GHCR on every release with SBOM and SLSA provenance attestation.

**Architecture:** New `container` job in the release workflow builds the two-stage image (apko + Docker prebake) for amd64/arm64, pushes to `ghcr.io/dakaneye/claude-sandbox`, signs with keyless Sigstore, attaches SBOM via syft, and attests SLSA provenance. Go code updated to pull from GHCR before falling back to local build.

**Tech Stack:** GitHub Actions, Docker Buildx, apko, cosign, syft, Go

**Spec:** `docs/specs/2026-03-27-container-image-release-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `.github/workflows/release.yml` | Add container build/push/sign/sbom/attest job |
| `internal/container/container.go` | Update `DefaultImage`, add `GHCRImage` const |
| `internal/container/build.go` | Add `Pull()` function |
| `internal/container/container_test.go` | Update tests for new image constants |
| `internal/cli/execute.go` | Update image resolution: local → pull → build |
| `README.md` | Add docker pull, verification, SBOM sections |

---

### Task 1: Add container job to release workflow

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Update permissions**

In `.github/workflows/release.yml`, replace:

```yaml
permissions:
  contents: write
```

With:

```yaml
permissions:
  contents: write
  packages: write
  id-token: write
```

- [ ] **Step 2: Add container job after the release job**

Add the following job to `.github/workflows/release.yml` after the `release` job:

```yaml
  container:
    runs-on: ubuntu-latest
    needs: release
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4
        with:
          fetch-depth: 0

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@b5ca514318bd6ebac0fb2aedd5d36ec1b5c232a2 # v3

      - name: Login to GHCR
        uses: docker/login-action@74a5d142397b4f367a81961eba4e8cd7edddf772 # v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Set up Go
        uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5
        with:
          go-version-file: go.mod

      - name: Install apko
        run: go install chainguard.dev/apko@latest

      - name: Extract version from tag
        id: version
        run: |
          VERSION=${GITHUB_REF_NAME#v}
          MAJOR=${VERSION%%.*}
          echo "version=$VERSION" >> "$GITHUB_OUTPUT"
          echo "major=$MAJOR" >> "$GITHUB_OUTPUT"

      - name: Build base image (amd64)
        run: |
          mkdir -p /tmp/build
          cp internal/container/configs/apko.yaml /tmp/build/
          cp internal/container/configs/prebake.dockerfile /tmp/build/
          apko build /tmp/build/apko.yaml claude-sandbox:base /tmp/build/base-amd64.tar --arch amd64

      - name: Build base image (arm64)
        run: |
          apko build /tmp/build/apko.yaml claude-sandbox:base /tmp/build/base-arm64.tar --arch arm64

      - name: Load base images
        run: |
          docker load -i /tmp/build/base-amd64.tar
          docker load -i /tmp/build/base-arm64.tar

      - name: Build and push multi-arch image
        run: |
          IMAGE=ghcr.io/dakaneye/claude-sandbox
          VERSION=${{ steps.version.outputs.version }}
          MAJOR=${{ steps.version.outputs.major }}

          # Build amd64
          docker build \
            -t "${IMAGE}:${VERSION}-amd64" \
            --build-arg BASE_IMAGE=claude-sandbox:base-amd64 \
            --label "org.opencontainers.image.source=https://github.com/dakaneye/claude-sandbox" \
            --label "org.opencontainers.image.version=${VERSION}" \
            --label "org.opencontainers.image.revision=${{ github.sha }}" \
            -f /tmp/build/prebake.dockerfile \
            /tmp/build
          docker push "${IMAGE}:${VERSION}-amd64"

          # Build arm64
          docker build \
            -t "${IMAGE}:${VERSION}-arm64" \
            --build-arg BASE_IMAGE=claude-sandbox:base-arm64 \
            --platform linux/arm64 \
            --label "org.opencontainers.image.source=https://github.com/dakaneye/claude-sandbox" \
            --label "org.opencontainers.image.version=${VERSION}" \
            --label "org.opencontainers.image.revision=${{ github.sha }}" \
            -f /tmp/build/prebake.dockerfile \
            /tmp/build
          docker push "${IMAGE}:${VERSION}-arm64"

          # Create and push multi-arch manifest
          for TAG in "${VERSION}" "${MAJOR}" "latest"; do
            docker manifest create "${IMAGE}:${TAG}" \
              "${IMAGE}:${VERSION}-amd64" \
              "${IMAGE}:${VERSION}-arm64"
            docker manifest push "${IMAGE}:${TAG}"
          done

      - name: Install cosign
        uses: sigstore/cosign-installer@3454372f43399081ed03b604cb2d7369c5f59764 # v3

      - name: Install syft
        uses: anchore/sbom-action/download-syft@e11c554f704a0b820cbf8c51673f6945e0731532 # v0

      - name: Sign image
        run: |
          IMAGE=ghcr.io/dakaneye/claude-sandbox
          VERSION=${{ steps.version.outputs.version }}
          DIGEST=$(docker manifest inspect "${IMAGE}:${VERSION}" -v | jq -r '.digest // .[0].Descriptor.digest')
          cosign sign --yes "${IMAGE}@${DIGEST}"

      - name: Generate and attach SBOM
        run: |
          IMAGE=ghcr.io/dakaneye/claude-sandbox
          VERSION=${{ steps.version.outputs.version }}
          DIGEST=$(docker manifest inspect "${IMAGE}:${VERSION}" -v | jq -r '.digest // .[0].Descriptor.digest')
          syft "${IMAGE}:${VERSION}" -o spdx-json > /tmp/sbom.spdx.json
          cosign attach sbom --sbom /tmp/sbom.spdx.json "${IMAGE}@${DIGEST}"

      - name: Attest SLSA provenance
        run: |
          IMAGE=ghcr.io/dakaneye/claude-sandbox
          VERSION=${{ steps.version.outputs.version }}
          DIGEST=$(docker manifest inspect "${IMAGE}:${VERSION}" -v | jq -r '.digest // .[0].Descriptor.digest')
          cosign attest --yes --type slsaprovenance --predicate <(cat <<PRED
          {
            "buildType": "https://github.com/dakaneye/claude-sandbox/container-build@v1",
            "builder": { "id": "https://github.com/dakaneye/claude-sandbox/actions/runs/${{ github.run_id }}" },
            "invocation": {
              "configSource": {
                "uri": "git+https://github.com/dakaneye/claude-sandbox@${{ github.ref }}",
                "digest": { "sha1": "${{ github.sha }}" },
                "entryPoint": ".github/workflows/release.yml"
              }
            },
            "metadata": {
              "buildStartedOn": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
              "completeness": { "parameters": true, "environment": true, "materials": true }
            },
            "materials": [
              { "uri": "git+https://github.com/dakaneye/claude-sandbox@${{ github.ref }}", "digest": { "sha1": "${{ github.sha }}" } }
            ]
          }
          PRED
          ) "${IMAGE}@${DIGEST}"
```

- [ ] **Step 3: Validate YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"`
Expected: No output (valid YAML)

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(release): add container image build, push, sign, SBOM, and attestation"
```

---

### Task 2: Add Pull function and update image constants

**Files:**
- Modify: `internal/container/container.go:14-23`
- Modify: `internal/container/build.go`
- Test: `internal/container/container_test.go`

- [ ] **Step 1: Write failing test for Pull**

Add to `internal/container/container_test.go`:

```go
func TestGHCRImageConstant(t *testing.T) {
	if GHCRImage == "" {
		t.Error("GHCRImage should not be empty")
	}
	if !strings.HasPrefix(GHCRImage, "ghcr.io/") {
		t.Errorf("GHCRImage should start with ghcr.io/, got: %s", GHCRImage)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/container/ -run TestGHCRImageConstant -v`
Expected: FAIL — `GHCRImage` not defined

- [ ] **Step 3: Add GHCRImage constant and Pull function**

In `internal/container/container.go`, update the const block:

```go
const (
	// DefaultImage is the local sandbox container image tag.
	DefaultImage = "claude-sandbox:latest"

	// GHCRImage is the public container image on GitHub Container Registry.
	GHCRImage = "ghcr.io/dakaneye/claude-sandbox:latest"

	// HistoryVolumeName is the Docker volume for Claude history persistence.
	HistoryVolumeName = "claude-sandbox-history"

	// ContainerNamePrefix is the prefix for sandbox container names.
	ContainerNamePrefix = "claude-sandbox-"
)
```

In `internal/container/build.go`, add the `Pull` function:

```go
// Pull pulls the sandbox image from GHCR and tags it as the default local image.
func Pull(w io.Writer) error {
	if err := CheckDocker(); err != nil {
		return err
	}

	fmt.Fprintf(w, "Pulling %s...\n", GHCRImage)
	cmd := exec.Command("docker", "pull", GHCRImage)
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pull image: %w", err)
	}

	// Tag as local default so Run() can use it
	tagCmd := exec.Command("docker", "tag", GHCRImage, DefaultImage)
	if err := tagCmd.Run(); err != nil {
		return fmt.Errorf("tag image: %w", err)
	}

	fmt.Fprintf(w, "Image ready: %s\n", DefaultImage)
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/container/ -run TestGHCRImageConstant -v`
Expected: PASS

- [ ] **Step 5: Run build**

Run: `go build ./...`
Expected: Clean

- [ ] **Step 6: Commit**

```bash
git add internal/container/container.go internal/container/build.go internal/container/container_test.go
git commit -m "feat(container): add GHCRImage constant and Pull function"
```

---

### Task 3: Update execute.go image resolution

**Files:**
- Modify: `internal/cli/execute.go:77-84`

- [ ] **Step 1: Replace the auto-build block**

In `internal/cli/execute.go`, replace lines 77-84:

```go
	// Auto-build container image if missing
	if !container.ImageExists(container.DefaultImage) {
		cmd.Println("Container image not found. Building...")
		if err := container.Build(cmd.OutOrStdout(), false); err != nil {
			return fmt.Errorf("build image: %w", err)
		}
		cmd.Println()
	}
```

With:

```go
	// Resolve container image: local → pull → build
	if !container.ImageExists(container.DefaultImage) {
		cmd.Println("Container image not found. Pulling from GHCR...")
		if err := container.Pull(cmd.OutOrStdout()); err != nil {
			cmd.PrintErrf("Pull failed: %v\n", err)
			cmd.Println("Falling back to local build...")
			if err := container.Build(cmd.OutOrStdout(), false); err != nil {
				return fmt.Errorf("build image: %w", err)
			}
		}
		cmd.Println()
	}
```

- [ ] **Step 2: Run build and tests**

Run: `go build ./... && go test ./internal/cli/ -count=1 2>&1 | tail -5`
Expected: Clean build, all tests pass

- [ ] **Step 3: Commit**

```bash
git add internal/cli/execute.go
git commit -m "feat(execute): pull image from GHCR before falling back to local build"
```

---

### Task 4: Update README with pull, verification, and SBOM

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update the container image section**

In `README.md`, replace the existing container build instruction block. Find:

```markdown
**Build the container image** (required before first `execute`):

```bash
claude-sandbox build
```

This uses apko to build a minimal container with Claude CLI, pre-configured settings, and the review-code skill.
```

Replace with:

```markdown
**Container image** (auto-pulled on first `execute`):

```bash
# Pulled automatically, or manually:
docker pull ghcr.io/dakaneye/claude-sandbox:latest

# Or build locally (requires apko):
claude-sandbox build
```
```

- [ ] **Step 2: Add Verification section before License**

Find `## License` and add this section before it:

```markdown
## Image Verification

The container image is signed with [Sigstore](https://sigstore.dev) keyless signing and includes an SBOM and SLSA provenance attestation.

```bash
# Verify image signature
cosign verify ghcr.io/dakaneye/claude-sandbox:latest \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/dakaneye/claude-sandbox'

# Download SBOM
cosign download sbom ghcr.io/dakaneye/claude-sandbox:latest | jq .

# Verify SLSA provenance
cosign verify-attestation ghcr.io/dakaneye/claude-sandbox:latest \
  --type slsaprovenance \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/dakaneye/claude-sandbox'
```

```

- [ ] **Step 3: Remove apko from Prerequisites**

In the Prerequisites section, remove the apko line since it's no longer required for basic usage. Change:

```markdown
- [Docker](https://docs.docker.com/get-docker/)
- [apko](https://github.com/chainguard-dev/apko) (container image build)
```

To:

```markdown
- [Docker](https://docs.docker.com/get-docker/)
```

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: add image pull, verification, and SBOM instructions to README"
```

---

### Task 5: Quality gates and validation

**Files:** None (verification only)

- [ ] **Step 1: Build**

Run: `make build`
Expected: PASS

- [ ] **Step 2: Lint**

Run: `golangci-lint run ./...`
Expected: 0 issues

- [ ] **Step 3: Tests**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 4: Tidy**

Run: `go mod tidy && git diff --exit-code go.mod go.sum`
Expected: No changes

- [ ] **Step 5: Validate workflow YAML**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"`
Expected: No error

- [ ] **Step 6: Verify workflow will trigger correctly**

Run: `grep -A2 "^on:" .github/workflows/release.yml`
Expected: Shows `push: tags: - "v*"` trigger
