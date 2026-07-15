#!/bin/bash
# LunaBox Build Script
# Usage: ./scripts/build.sh [portable|installer|all] [version] [amd64|arm64]

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BUILD_MODE="${1:-all}"
VERSION_ARG="${2:-}"
TARGET_ARCH="${3:-}"

case "$BUILD_MODE" in
    portable|installer|all) ;;
    *)
        echo "Unknown build mode: $BUILD_MODE"
        echo "Usage: ./scripts/build.sh [portable|installer|all] [version] [amd64|arm64]"
        exit 1
        ;;
esac

if [[ "$VERSION_ARG" == "amd64" || "$VERSION_ARG" == "x64" ]]; then
    TARGET_ARCH="amd64"
    VERSION_ARG=""
elif [[ "$VERSION_ARG" == "arm64" || "$VERSION_ARG" == "aarch64" ]]; then
    TARGET_ARCH="arm64"
    VERSION_ARG=""
fi

if [ -z "$TARGET_ARCH" ]; then
    case "$(uname -m)" in
        arm64|aarch64) TARGET_ARCH="arm64" ;;
        x86_64|amd64) TARGET_ARCH="amd64" ;;
        *)
            echo "ERROR: unsupported host architecture: $(uname -m)"
            exit 1
            ;;
    esac
fi

case "$TARGET_ARCH" in
    arm64|aarch64) TARGET_ARCH="arm64" ;;
    amd64|x64|x86_64) TARGET_ARCH="amd64" ;;
    *)
        echo "Unknown target architecture: $TARGET_ARCH"
        echo "Usage: ./scripts/build.sh [portable|installer|all] [version] [amd64|arm64]"
        exit 1
        ;;
esac

WAILS_PLATFORM=""
GOOS_VALUE="$(go env GOOS)"
case "$GOOS_VALUE" in
    darwin) WAILS_PLATFORM="darwin/$TARGET_ARCH" ;;
    linux) WAILS_PLATFORM="linux/$TARGET_ARCH" ;;
    windows) WAILS_PLATFORM="windows/$TARGET_ARCH" ;;
esac
WAILS_VERBOSITY="${WAILS_VERBOSITY:-0}"

BUILD_ENV_FILE=""
if [ -f ".env.build" ]; then
    BUILD_ENV_FILE=".env.build"
elif [ -f ".env" ]; then
    BUILD_ENV_FILE=".env"
fi

read_build_env() {
    local line key value
    [ -n "$BUILD_ENV_FILE" ] || return

    while IFS= read -r line || [ -n "$line" ]; do
        line="${line%$'\r'}"
        case "$line" in
            ""|\#*) continue ;;
            export\ *) line="${line#export }" ;;
        esac
        key="${line%%=*}"
        value="${line#*=}"
        key="$(printf '%s' "$key" | xargs)"
        case "$key" in
            LUNABOX_BANGUMI_CLIENT_ID)
                if [ -z "${LUNABOX_BANGUMI_CLIENT_ID:-}" ]; then
                    LUNABOX_BANGUMI_CLIENT_ID="$(trim_env_value "$value")"
                    export LUNABOX_BANGUMI_CLIENT_ID
                fi
                ;;
            LUNABOX_BANGUMI_CLIENT_SECRET)
                if [ -z "${LUNABOX_BANGUMI_CLIENT_SECRET:-}" ]; then
                    LUNABOX_BANGUMI_CLIENT_SECRET="$(trim_env_value "$value")"
                    export LUNABOX_BANGUMI_CLIENT_SECRET
                fi
                ;;
            LUNABOX_HIKARINAGI_CLIENT_ID)
                if [ -z "${LUNABOX_HIKARINAGI_CLIENT_ID:-}" ]; then
                    LUNABOX_HIKARINAGI_CLIENT_ID="$(trim_env_value "$value")"
                    export LUNABOX_HIKARINAGI_CLIENT_ID
                fi
                ;;
            LUNABOX_HIKARINAGI_CLIENT_SECRET)
                if [ -z "${LUNABOX_HIKARINAGI_CLIENT_SECRET:-}" ]; then
                    LUNABOX_HIKARINAGI_CLIENT_SECRET="$(trim_env_value "$value")"
                    export LUNABOX_HIKARINAGI_CLIENT_SECRET
                fi
                ;;
            LUNABOX_TOUCHGAL_TOKEN)
                if [ -z "${LUNABOX_TOUCHGAL_TOKEN:-}" ]; then
                    LUNABOX_TOUCHGAL_TOKEN="$(trim_env_value "$value")"
                    export LUNABOX_TOUCHGAL_TOKEN
                fi
                ;;
            LUNABOX_UMBRA_CLIENT_ID)
                if [ -z "${LUNABOX_UMBRA_CLIENT_ID:-}" ]; then
                    LUNABOX_UMBRA_CLIENT_ID="$(trim_env_value "$value")"
                    export LUNABOX_UMBRA_CLIENT_ID
                fi
                ;;
            LUNABOX_UMBRA_REGISTRATION_TOKEN)
                if [ -z "${LUNABOX_UMBRA_REGISTRATION_TOKEN:-}" ]; then
                    LUNABOX_UMBRA_REGISTRATION_TOKEN="$(trim_env_value "$value")"
                    export LUNABOX_UMBRA_REGISTRATION_TOKEN
                fi
                ;;
        esac
    done < "$BUILD_ENV_FILE"
}

trim_env_value() {
    local value="$1"
    value="$(printf '%s' "$value" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    if [[ "$value" == \"*\" && "$value" == *\" ]]; then
        value="${value:1:${#value}-2}"
    elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
        value="${value:1:${#value}-2}"
    fi
    printf '%s' "$value"
}

ldflag_set() {
    local symbol="$1"
    local value="$2"
    if [[ "$value" == *"'"* ]]; then
        echo "ERROR: ldflag value for $symbol contains a single quote, which this build script cannot safely pass."
        exit 1
    fi
    printf -- "-X '%s=%s'" "$symbol" "$value"
}

read_build_env

if [ -n "$VERSION_ARG" ]; then
    VERSION="$VERSION_ARG"
else
    VERSION="$(git describe --tags --abbrev=0 2>/dev/null || true)"
    [ -n "$VERSION" ] || VERSION="v1.0.0"
fi
VERSION="${VERSION#v}"

GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || true)"
[ -n "$GIT_COMMIT" ] || GIT_COMMIT="unknown"
BUILD_TIME="$(date '+%Y-%m-%d %H:%M:%S')"

LDFLAGS_BANGUMI=""
BANGUMI_OAUTH_STATUS="disabled"
if [ -n "${LUNABOX_BANGUMI_CLIENT_ID:-}" ]; then
    if [ -z "${LUNABOX_BANGUMI_CLIENT_SECRET:-}" ]; then
        echo "ERROR: LUNABOX_BANGUMI_CLIENT_SECRET is missing."
        exit 1
    fi
    LDFLAGS_BANGUMI=" $(ldflag_set "lunabox/internal/version.BangumiOAuthClientID" "$LUNABOX_BANGUMI_CLIENT_ID") $(ldflag_set "lunabox/internal/version.BangumiOAuthClientSecret" "$LUNABOX_BANGUMI_CLIENT_SECRET")"
    BANGUMI_OAUTH_STATUS="enabled"
elif [ -n "${LUNABOX_BANGUMI_CLIENT_SECRET:-}" ]; then
    echo "ERROR: LUNABOX_BANGUMI_CLIENT_ID is missing."
    exit 1
fi

LDFLAGS_HIKARINAGI=""
HIKARINAGI_OAUTH_STATUS="disabled"
if [ -n "${LUNABOX_HIKARINAGI_CLIENT_ID:-}" ]; then
    if [ -z "${LUNABOX_HIKARINAGI_CLIENT_SECRET:-}" ]; then
        echo "ERROR: LUNABOX_HIKARINAGI_CLIENT_SECRET is missing."
        exit 1
    fi
    LDFLAGS_HIKARINAGI=" $(ldflag_set "lunabox/internal/version.HikarinagiOAuthClientID" "$LUNABOX_HIKARINAGI_CLIENT_ID") $(ldflag_set "lunabox/internal/version.HikarinagiOAuthClientSecret" "$LUNABOX_HIKARINAGI_CLIENT_SECRET")"
    HIKARINAGI_OAUTH_STATUS="enabled"
elif [ -n "${LUNABOX_HIKARINAGI_CLIENT_SECRET:-}" ]; then
    echo "ERROR: LUNABOX_HIKARINAGI_CLIENT_ID is missing."
    exit 1
fi

LDFLAGS_TOUCHGAL=""
TOUCHGAL_TOKEN_STATUS="disabled"
if [ -n "${LUNABOX_TOUCHGAL_TOKEN:-}" ]; then
    LDFLAGS_TOUCHGAL=" $(ldflag_set "lunabox/internal/version.TouchGalAPIToken" "$LUNABOX_TOUCHGAL_TOKEN")"
    TOUCHGAL_TOKEN_STATUS="enabled"
fi

LDFLAGS_UMBRA=""
UMBRA_REGISTRATION_STATUS="disabled"
if [ -n "${LUNABOX_UMBRA_CLIENT_ID:-}" ]; then
    if [ -z "${LUNABOX_UMBRA_REGISTRATION_TOKEN:-}" ]; then
        echo "ERROR: LUNABOX_UMBRA_CLIENT_ID and LUNABOX_UMBRA_REGISTRATION_TOKEN must be configured together."
        exit 1
    fi
    LDFLAGS_UMBRA=" $(ldflag_set "lunabox/internal/version.UmbraOAuthClientID" "$LUNABOX_UMBRA_CLIENT_ID") $(ldflag_set "lunabox/internal/version.UmbraRegistrationToken" "$LUNABOX_UMBRA_REGISTRATION_TOKEN")"
    UMBRA_REGISTRATION_STATUS="enabled"
elif [ -n "${LUNABOX_UMBRA_REGISTRATION_TOKEN:-}" ]; then
    echo "ERROR: LUNABOX_UMBRA_CLIENT_ID and LUNABOX_UMBRA_REGISTRATION_TOKEN must be configured together."
    exit 1
fi

LDFLAGS_BASE="-s -w $(ldflag_set "lunabox/internal/version.Version" "$VERSION") $(ldflag_set "lunabox/internal/version.GitCommit" "$GIT_COMMIT") $(ldflag_set "lunabox/internal/version.BuildTime" "$BUILD_TIME")$LDFLAGS_BANGUMI$LDFLAGS_HIKARINAGI$LDFLAGS_TOUCHGAL$LDFLAGS_UMBRA"
LDFLAGS_PORTABLE="$LDFLAGS_BASE $(ldflag_set "lunabox/internal/version.BuildMode" "portable")"
LDFLAGS_INSTALLER="$LDFLAGS_BASE $(ldflag_set "lunabox/internal/version.BuildMode" "installer")"

MAC_SEVENZIP_SOURCE="lib/macarm64/7z/7zz"
MAC_CLI_BUILD="build/bin/lunacli"
MAC_DMG_STAGING_DIR="build/dmg"

run_wails_build() {
	local ldflags="$1"
	shift
	local args=(build -v "$WAILS_VERBOSITY")
    if [ -n "$WAILS_PLATFORM" ]; then
        args+=(-platform "$WAILS_PLATFORM")
    fi
    args+=(-ldflags "$ldflags")
    args+=("$@")
	wails "${args[@]}"
}

clean_macos_build_outputs() {
	if [ "$(uname -s)" != "Darwin" ]; then
		return
	fi
	rm -rf build/bin/LunaBox.app "$MAC_DMG_STAGING_DIR"
}

find_macos_app() {
    local preferred="$1"
    if [ -n "$preferred" ] && [ -d "$preferred" ]; then
        printf '%s' "$preferred"
        return
    fi
    if [ -d "build/bin/LunaBox.app" ]; then
        printf '%s' "build/bin/LunaBox.app"
        return
    fi
    find build/bin -maxdepth 1 -name '*.app' -type d | sort | head -n 1
}

build_cli() {
    local ldflags="$1"
    local output="$2"

    echo "Building CLI Version..."
    echo "----------------------------------------"
    GOOS="$GOOS_VALUE" GOARCH="$TARGET_ARCH" go build -trimpath -ldflags "$ldflags" -o "$output" ./cmd/lunacli
    chmod 755 "$output"
    echo "CLI build completed: $output"
    echo
}

copy_macos_runtime_tools() {
    local app_path="$1"

    if [ "$(uname -s)" != "Darwin" ]; then
        return
    fi
    if [ "$TARGET_ARCH" != "arm64" ]; then
        echo "WARNING: bundled 7zz is only available for macOS arm64; skipping 7zz copy for $TARGET_ARCH."
    elif [ ! -f "$MAC_SEVENZIP_SOURCE" ]; then
        echo "ERROR: bundled 7zz not found at $MAC_SEVENZIP_SOURCE"
        exit 1
    fi

    app_path="$(find_macos_app "$app_path")"
    if [ -z "$app_path" ]; then
        echo "ERROR: no .app bundle found under build/bin"
        exit 1
    fi

    mkdir -p "$app_path/Contents/Resources/bin"
    if [ "$TARGET_ARCH" = "arm64" ]; then
        cp "$MAC_SEVENZIP_SOURCE" "$app_path/Contents/Resources/bin/7zz"
        chmod 755 "$app_path/Contents/Resources/bin/7zz"
        echo "Bundled 7zz copied to $app_path/Contents/Resources/bin/7zz"
    fi
    if [ -f "$MAC_CLI_BUILD" ]; then
        cp "$MAC_CLI_BUILD" "$app_path/Contents/Resources/bin/lunacli"
        chmod 755 "$app_path/Contents/Resources/bin/lunacli"
        echo "CLI copied to $app_path/Contents/Resources/bin/lunacli"
    fi
}

create_macos_dmg() {
    local app_path="$1"
    app_path="$(find_macos_app "$app_path")"
    if [ -z "$app_path" ]; then
        echo "ERROR: no .app bundle found under build/bin"
        exit 1
    fi
    if ! command -v hdiutil >/dev/null 2>&1; then
        echo "ERROR: hdiutil is required to create macOS DMG packages."
        exit 1
    fi

    local dmg_name="LunaBox-${VERSION}-macos-${TARGET_ARCH}.dmg"
    local dmg_path="build/bin/$dmg_name"
    local staging_dir="$MAC_DMG_STAGING_DIR/LunaBox-${VERSION}-macos-${TARGET_ARCH}"

    rm -rf "$staging_dir"
    mkdir -p "$staging_dir"
    ditto "$app_path" "$staging_dir/LunaBox.app"
    ln -s /Applications "$staging_dir/Applications"

    rm -f "$dmg_path"
    hdiutil create -volname "LunaBox" -srcfolder "$staging_dir" -ov -format UDZO "$dmg_path" >/dev/null
    rm -rf "$staging_dir"
    echo "Created: $dmg_path"
}

echo "========================================"
echo "LunaBox Build Script"
echo "Build Mode: $BUILD_MODE"
echo "Target Arch: $TARGET_ARCH"
echo "Version: $VERSION"
echo "Commit: $GIT_COMMIT"
if [ -n "$BUILD_ENV_FILE" ]; then echo "Build Env File: $BUILD_ENV_FILE"; fi
echo "Bangumi OAuth Injection: $BANGUMI_OAUTH_STATUS"
echo "Hikarinagi OAuth Injection: $HIKARINAGI_OAUTH_STATUS"
echo "TouchGAL Token Injection: $TOUCHGAL_TOKEN_STATUS"
echo "Umbra Registration Token Injection: $UMBRA_REGISTRATION_STATUS"
if [ "$(uname -s)" = "Darwin" ] && [ -f "$MAC_SEVENZIP_SOURCE" ]; then echo "Bundled 7zz: $MAC_SEVENZIP_SOURCE"; fi
echo "========================================"
echo

build_portable() {
    if [ "$(uname -s)" = "Darwin" ]; then
        echo "macOS portable package is not supported. Use './scripts/build.sh installer' to create a DMG."
        exit 1
    fi

    echo "[1/3] Building Portable GUI Version..."
    echo "----------------------------------------"
    local output_name="LunaBox-${TARGET_ARCH}-portable"
    local app_path="build/bin/LunaBox.app"
    run_wails_build "$LDFLAGS_PORTABLE" -o "$output_name"
    echo "Portable GUI build completed: $app_path"
    echo

    echo "[2/3] Building Portable CLI Version..."
    build_cli "$LDFLAGS_PORTABLE" "$MAC_CLI_BUILD"

    echo "[3/3] Bundling Portable Runtime Tools..."
    echo "----------------------------------------"
    copy_macos_runtime_tools "$app_path"
    echo
}

build_installer() {
    echo "[1/2] Building Installer CLI Version..."
    build_cli "$LDFLAGS_INSTALLER" "$MAC_CLI_BUILD"

    echo "[2/2] Building Installer GUI Version..."
    echo "----------------------------------------"
    case "$(uname -s)" in
		Darwin*)
			local output_name="LunaBox-${TARGET_ARCH}-installer"
			local app_path="build/bin/LunaBox.app"
			clean_macos_build_outputs
			run_wails_build "$LDFLAGS_INSTALLER" -o "$output_name"
			copy_macos_runtime_tools "$app_path"
			create_macos_dmg "$app_path"
            echo "macOS build completed: $app_path"
            ;;
        Linux*)
            run_wails_build "$LDFLAGS_INSTALLER"
            echo "Linux build completed"
            ;;
        MINGW*|CYGWIN*|MSYS*)
            run_wails_build "$LDFLAGS_INSTALLER" -nsis
            echo "Windows installer build completed"
            ;;
        *)
            echo "Unknown OS, building without installer..."
            run_wails_build "$LDFLAGS_INSTALLER"
            ;;
    esac
    echo
}

case "$BUILD_MODE" in
    portable)
        build_portable
        ;;
    installer)
        build_installer
        ;;
    all)
        echo "Building all versions..."
        echo
        if [ "$(uname -s)" != "Darwin" ]; then
            build_portable
        fi
        build_installer
        ;;
esac

echo "========================================"
echo "Build completed successfully!"
echo "========================================"
echo
if [ "$(uname -s)" = "Darwin" ]; then
    echo "Output files:"
    echo "  - DMG: build/bin/LunaBox-${VERSION}-macos-${TARGET_ARCH}.dmg"
    echo "  - App bundle: build/bin/LunaBox.app"
fi
echo "Installer version: Data stored in user config directory"
echo
