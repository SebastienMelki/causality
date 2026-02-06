#!/usr/bin/env bash
#
# gomobile-build.sh - Build Causality mobile SDK using gomobile bind.
#
# Usage:
#   ./scripts/gomobile-build.sh [ios|android|all]
#
# Prerequisites:
#   - Go >= 1.24
#   - gomobile installed: go install golang.org/x/mobile/cmd/gomobile@latest
#   - For iOS: Xcode + Command Line Tools
#   - For Android: Android NDK (ANDROID_NDK_HOME set)
#
# Output:
#   - iOS:     build/mobile/Causality.xcframework
#   - Android: build/mobile/causality.aar

set -euo pipefail

# Project root (script location relative)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Output directory
BUILD_DIR="${PROJECT_ROOT}/build/mobile"

# SDK package to bind
SDK_PACKAGE="./sdk/mobile"

# iOS settings
IOS_PREFIX="CAU"
IOS_OUTPUT="${BUILD_DIR}/Causality.xcframework"

# Android settings
ANDROID_API=21
ANDROID_JAVA_PKG="io.causality.mobile"
ANDROID_OUTPUT="${BUILD_DIR}/causality.aar"

# Minimum required Go version
MIN_GO_VERSION="1.24"

# ---- Helper functions ----

log() {
    echo "[gomobile-build] $*"
}

error() {
    echo "[gomobile-build] ERROR: $*" >&2
    exit 1
}

check_go_version() {
    if ! command -v go &> /dev/null; then
        error "Go is not installed or not in PATH"
    fi

    local go_version
    go_version=$(go version | grep -oP 'go(\d+\.\d+)' | head -1 | sed 's/go//')
    if [ -z "$go_version" ]; then
        # Fallback for different go version output formats
        go_version=$(go version | awk '{print $3}' | sed 's/go//' | cut -d. -f1,2)
    fi

    local major minor min_major min_minor
    major=$(echo "$go_version" | cut -d. -f1)
    minor=$(echo "$go_version" | cut -d. -f2)
    min_major=$(echo "$MIN_GO_VERSION" | cut -d. -f1)
    min_minor=$(echo "$MIN_GO_VERSION" | cut -d. -f2)

    if [ "$major" -lt "$min_major" ] || { [ "$major" -eq "$min_major" ] && [ "$minor" -lt "$min_minor" ]; }; then
        error "Go ${MIN_GO_VERSION}+ required, found go${go_version}"
    fi

    log "Go version: go${go_version}"
}

check_gomobile() {
    if ! command -v gomobile &> /dev/null; then
        error "gomobile not found. Install with: go install golang.org/x/mobile/cmd/gomobile@latest"
    fi
    log "gomobile found: $(which gomobile)"
}

init_gomobile() {
    log "Initializing gomobile..."
    gomobile init
}

build_ios() {
    log "Building iOS SDK..."
    log "  Target: ios"
    log "  Prefix: ${IOS_PREFIX}"
    log "  Output: ${IOS_OUTPUT}"

    mkdir -p "${BUILD_DIR}"

    cd "${PROJECT_ROOT}"
    gomobile bind \
        -target ios \
        -prefix "${IOS_PREFIX}" \
        -o "${IOS_OUTPUT}" \
        "${SDK_PACKAGE}"

    log "iOS build complete: ${IOS_OUTPUT}"
    log "  Size: $(du -sh "${IOS_OUTPUT}" | cut -f1)"
}

build_android() {
    log "Building Android SDK..."
    log "  Target: android (API ${ANDROID_API})"
    log "  Java package: ${ANDROID_JAVA_PKG}"
    log "  Output: ${ANDROID_OUTPUT}"

    mkdir -p "${BUILD_DIR}"

    cd "${PROJECT_ROOT}"
    gomobile bind \
        -target android \
        -androidapi "${ANDROID_API}" \
        -javapkg "${ANDROID_JAVA_PKG}" \
        -o "${ANDROID_OUTPUT}" \
        "${SDK_PACKAGE}"

    log "Android build complete: ${ANDROID_OUTPUT}"
    log "  Size: $(du -sh "${ANDROID_OUTPUT}" | cut -f1)"
}

usage() {
    echo "Usage: $0 [ios|android|all]"
    echo ""
    echo "Build the Causality mobile SDK using gomobile bind."
    echo ""
    echo "Commands:"
    echo "  ios      Build iOS .xcframework"
    echo "  android  Build Android .aar"
    echo "  all      Build both iOS and Android (default)"
    echo ""
    echo "Output:"
    echo "  iOS:     ${IOS_OUTPUT}"
    echo "  Android: ${ANDROID_OUTPUT}"
}

# ---- Main ----

main() {
    local target="${1:-all}"

    case "$target" in
        ios)
            check_go_version
            check_gomobile
            init_gomobile
            build_ios
            ;;
        android)
            check_go_version
            check_gomobile
            init_gomobile
            build_android
            ;;
        all)
            check_go_version
            check_gomobile
            init_gomobile
            build_ios
            build_android
            log "All builds complete!"
            ;;
        -h|--help|help)
            usage
            exit 0
            ;;
        *)
            error "Unknown target: ${target}. Use ios, android, or all."
            ;;
    esac
}

main "$@"
