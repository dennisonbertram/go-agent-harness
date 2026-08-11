#!/usr/bin/env bash
# Sourceable, fail-closed runtime provenance checks for acceptance launchers.

acceptance_runtime_provenance_check() {
    local binary="$1"
    local expected_revision="$2"
    local artifact="$3"
    local build_info revision modified vcs digest

    if [ ! -x "${binary}" ]; then
        printf 'runtime provenance rejected: executable not found: %s\n' "${binary}" >&2
        return 1
    fi
    if [ -z "${expected_revision}" ]; then
        printf 'runtime provenance rejected: expected revision is empty\n' >&2
        return 1
    fi
    if ! build_info="$(go version -m "${binary}")"; then
        printf 'runtime provenance rejected: could not read go build info for %s\n' "${binary}" >&2
        return 1
    fi
    vcs="$(printf '%s\n' "${build_info}" | awk '$1 == "build" && $2 == "vcs=git" { print "git"; exit }')"
    revision="$(printf '%s\n' "${build_info}" | awk '$1 == "build" && $2 ~ /^vcs.revision=/ { sub(/^vcs.revision=/, "", $2); print $2; exit }')"
    modified="$(printf '%s\n' "${build_info}" | awk '$1 == "build" && $2 ~ /^vcs.modified=/ { sub(/^vcs.modified=/, "", $2); print $2; exit }')"
    if [ "${vcs}" != "git" ] || [ "${revision}" != "${expected_revision}" ] || [ "${modified}" != "false" ]; then
        printf 'runtime provenance rejected: expected clean git revision %s; got revision=%s modified=%s\n' \
            "${expected_revision}" "${revision:-missing}" "${modified:-missing}" >&2
        return 1
    fi
    if ! digest="$(shasum -a 256 "${binary}" | awk '{print $1}')" || [ -z "${digest}" ]; then
        printf 'runtime provenance rejected: could not calculate sha256 for %s\n' "${binary}" >&2
        return 1
    fi
    if ! mkdir -p "$(dirname "${artifact}")"; then
        printf 'runtime provenance rejected: could not create artifact directory for %s\n' "${artifact}" >&2
        return 1
    fi
    if ! BUILD_INFO="${build_info}" BINARY="${binary}" EXPECTED_REVISION="${expected_revision}" DIGEST="${digest}" \
        python3 -c 'import json, os; print(json.dumps({"binary": os.environ["BINARY"], "revision": os.environ["EXPECTED_REVISION"], "sha256": os.environ["DIGEST"], "build_info": os.environ["BUILD_INFO"]}, indent=2, sort_keys=True))' \
        > "${artifact}"; then
        printf 'runtime provenance rejected: could not write artifact %s\n' "${artifact}" >&2
        return 1
    fi
    printf 'runtime provenance accepted: revision=%s sha256=%s artifact=%s\n' "${expected_revision}" "${digest}" "${artifact}"
}

acceptance_runtime_require_clean_checkout() {
    local repo_root="$1"
    if [ -n "$(git -C "${repo_root}" status --porcelain)" ]; then
        printf 'runtime provenance rejected: requested checkout is dirty: %s\n' "${repo_root}" >&2
        return 1
    fi
}
