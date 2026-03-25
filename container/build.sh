#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_NAME="${IMAGE_NAME:-claude-sandbox}"
TAG="${TAG:-latest}"

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Build the claude-sandbox container image.

Options:
    --load          Load image into Docker after build
    --push REGISTRY Push to registry (e.g., ghcr.io/user)
    --arch ARCH     Build for specific arch (amd64, arm64)
    --no-prebake    Skip pre-baking Claude Code into image
    -h, --help      Show this help

Examples:
    $(basename "$0") --load
    $(basename "$0") --push ghcr.io/samueldacanay
EOF
}

main() {
    local load=false
    local push=""
    local arch=""
    local prebake=true

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --load) load=true; shift ;;
            --push) push="$2"; shift 2 ;;
            --arch) arch="$2"; shift 2 ;;
            --no-prebake) prebake=false; shift ;;
            -h|--help) usage; exit 0 ;;
            *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
        esac
    done

    echo "Building claude-sandbox base image..."

    local apko_args=("build" "${SCRIPT_DIR}/claude-sandbox.apko.yaml" "${IMAGE_NAME}:base" "${SCRIPT_DIR}/${IMAGE_NAME}-base.tar")

    if [[ -n "$arch" ]]; then
        apko_args+=("--arch" "$arch")
    fi

    apko "${apko_args[@]}"

    echo "Loading base image into Docker..."
    docker load < "${SCRIPT_DIR}/${IMAGE_NAME}-base.tar"

    if [[ "$prebake" == true ]]; then
        echo "Pre-baking Claude Code into image..."
        docker build -t "${IMAGE_NAME}:${TAG}" -f "${SCRIPT_DIR}/Dockerfile.prebake" "${SCRIPT_DIR}"
    else
        docker tag "${IMAGE_NAME}:base" "${IMAGE_NAME}:${TAG}"
    fi

    if [[ "$load" == true ]]; then
        echo "Image ready: ${IMAGE_NAME}:${TAG}"
    fi

    if [[ -n "$push" ]]; then
        local full_tag="${push}/${IMAGE_NAME}:${TAG}"
        echo "Pushing to ${full_tag}..."
        docker tag "${IMAGE_NAME}:${TAG}" "${full_tag}"
        docker push "${full_tag}"
    fi

    # Cleanup
    rm -f "${SCRIPT_DIR}/${IMAGE_NAME}-base.tar"

    echo "Done."
}

main "$@"
