#!/usr/bin/env bash
set -Eeuo pipefail
: "${RELEASE_TAG:?}" "${GITHUB_SHA:?}"
if metadata="$(gh release view "$RELEASE_TAG" --json isDraft,targetCommitish --jq '[.isDraft,.targetCommitish] | @tsv' 2>/dev/null)";then
  IFS=$'\t' read -r draft target <<< "$metadata"
  if [ "$draft" = false ];then
    echo 'Published release is immutable; use a new version for different assets.'
    exit 0
  fi
  if [ "$draft" != true ] || [ "$target" != "$GITHUB_SHA" ];then
    echo 'Existing draft belongs to a different source commit; refusing to overwrite.' >&2
    exit 1
  fi
else
  gh release create "$RELEASE_TAG" --target "$GITHUB_SHA" --draft --prerelease --title "Caddy native $RELEASE_TAG" --notes 'Native Linux amd64/arm64 preview; nodes need no Docker. Includes checksums, exact module/Go/customization sources, original notices and build provenance. CF/MTG connectors remain separate. Real host and network acceptance remains required.'
fi
# Clobber is restricted to an unpublished draft belonging to this exact commit.
gh release upload "$RELEASE_TAG" ./*.tar.gz ./SHA256SUMS ./build-info-*.json ./dependency-inventory-*.json --clobber
gh release edit "$RELEASE_TAG" --draft=false
