#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/.."
tmp="$(mktemp -d)";trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/bin"
export RELEASE_TEST_STATE="$tmp/state" GITHUB_SHA=fixture-commit RELEASE_TAG=v-fixture
cat > "$tmp/bin/gh" <<'EOF'
#!/usr/bin/env bash
set -eu
case "$1 $2" in
 'release view') test -f "$RELEASE_TEST_STATE" || exit 1;state=$(cat "$RELEASE_TEST_STATE");if [[ "$*" = *--json* ]];then if [ "$state" = published ];then printf 'false\tfixture-commit\n';else printf 'true\t%s\n' "${DRAFT_COMMIT:-fixture-commit}";fi;fi;;
 'release create') echo draft > "$RELEASE_TEST_STATE";;
 'release upload') if [ -f "$RELEASE_TEST_STATE.fail" ];then rm "$RELEASE_TEST_STATE.fail";exit 23;fi;touch "$RELEASE_TEST_STATE.assets";;
 'release edit') test -f "$RELEASE_TEST_STATE.assets";echo published > "$RELEASE_TEST_STATE";;
 *) exit 9;;
esac
EOF
chmod +x "$tmp/bin/gh";export PATH="$tmp/bin:$PATH"
if [ -f scripts/publish_release.sh ];then cp scripts/publish_release.sh "$tmp/publish.sh";else sed -n '/          if gh release view/,$p' .github/workflows/release.yml | sed 's/^          //' > "$tmp/publish.sh";fi
echo draft > "$RELEASE_TEST_STATE"
bash "$tmp/publish.sh"
test "$(cat "$RELEASE_TEST_STATE")" = published || { echo 'FAIL: incomplete draft was skipped';exit 1; }
echo draft > "$RELEASE_TEST_STATE";touch "$RELEASE_TEST_STATE.fail";rm "$RELEASE_TEST_STATE.assets"
if bash "$tmp/publish.sh";then echo 'FAIL: failed upload reported success';exit 1;fi
bash "$tmp/publish.sh";test "$(cat "$RELEASE_TEST_STATE")" = published
rm "$RELEASE_TEST_STATE.assets";bash "$tmp/publish.sh";test ! -e "$RELEASE_TEST_STATE.assets"
echo draft > "$RELEASE_TEST_STATE";export DRAFT_COMMIT=another-commit
if bash "$tmp/publish.sh";then echo 'FAIL: wrong source draft overwritten';exit 1;fi
echo 'PASS: interrupted drafts resume, published assets stay immutable, foreign drafts are refused'
