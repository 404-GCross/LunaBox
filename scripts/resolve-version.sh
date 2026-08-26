#!/usr/bin/env bash

set -euo pipefail

VERSION_ARG="${1:-}"

is_semver_like() {
    local value="${1#v}"
    [[ "$value" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?(\+[0-9A-Za-z][0-9A-Za-z.-]*)?$ ]]
}

git_short_commit() {
    local commit="${GITHUB_SHA:-}"
    if [[ -z "$commit" ]]; then
        commit="$(git rev-parse --short HEAD 2>/dev/null || true)"
    fi
    commit="${commit:0:7}"
    [[ -n "$commit" ]] || commit="unknown"
    printf '%s' "$commit"
}

git_commit_count() {
    local count
    count="$(git rev-list --count HEAD 2>/dev/null || true)"
    [[ -n "$count" ]] || count="0"
    printf '%s' "$count"
}

latest_base_version() {
    local tag base
    tag="$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)"
    base="${tag#v}"
    if ! is_semver_like "$base"; then
        base="1.0.0"
    fi
    printf '%s' "$base"
}

if [[ -n "$VERSION_ARG" ]] && is_semver_like "$VERSION_ARG"; then
    printf '%s\n' "${VERSION_ARG#v}"
    exit 0
fi

base_version="$(latest_base_version)"

if [[ -z "$VERSION_ARG" ]]; then
    printf '%s\n' "$base_version"
    exit 0
fi

printf '%s-dev.%s+%s\n' "$base_version" "$(git_commit_count)" "$(git_short_commit)"
