#!/usr/bin/env bash
set -euo pipefail

REPO="Obedience-Corp/camp-graph"
BINARY="camp-graph"
INSTALL_DIR="${INSTALL_DIR:-${GOBIN:-${HOME}/.local/bin}}"

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

detect_os() {
    case "$(uname -s)" in
        Darwin) echo "macOS" ;;
        Linux) echo "linux" ;;
        *)
            echo "unsupported operating system: $(uname -s)" >&2
            exit 1
            ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "x86_64" ;;
        arm64|aarch64) echo "arm64" ;;
        *)
            echo "unsupported architecture: $(uname -m)" >&2
            exit 1
            ;;
    esac
}

resolve_tag() {
    if [[ -n "${VERSION:-}" ]]; then
        echo "${VERSION}"
        return
    fi

    local tag
    tag="$(
        curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
            sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
            head -n1
    )"

    if [[ -z "${tag}" ]]; then
        echo "could not determine latest release tag for ${REPO}" >&2
        exit 1
    fi

    echo "${tag}"
}

require_tools() {
    if ! command_exists curl; then
        echo "curl is required" >&2
        exit 1
    fi
    if ! command_exists tar; then
        echo "tar is required" >&2
        exit 1
    fi
    if ! command_exists install; then
        echo "install is required" >&2
        exit 1
    fi
}

main() {
    require_tools

    local os arch tag version archive url tmp_dir
    os="$(detect_os)"
    arch="$(detect_arch)"
    tag="$(resolve_tag)"
    version="${tag#v}"
    archive="${BINARY}-${version}-${os}-${arch}.tar.gz"
    url="https://github.com/${REPO}/releases/download/${tag}/${archive}"
    tmp_dir="$(mktemp -d)"
    trap 'rm -rf "${tmp_dir}"' EXIT

    echo "Downloading ${url}"
    curl -fsSL "${url}" -o "${tmp_dir}/${archive}"
    tar -xzf "${tmp_dir}/${archive}" -C "${tmp_dir}"

    mkdir -p "${INSTALL_DIR}"
    install -m 0755 "${tmp_dir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

    echo "Installed ${BINARY} ${tag} to ${INSTALL_DIR}/${BINARY}"
    echo "Verify with: ${BINARY} version"
    echo "Use through camp with: camp graph build"

    case ":${PATH}:" in
        *:"${INSTALL_DIR}":*) ;;
        *)
            echo ""
            echo "Add ${INSTALL_DIR} to your PATH if needed."
            ;;
    esac
}

main "$@"
