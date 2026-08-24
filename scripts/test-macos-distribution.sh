#!/usr/bin/env bash
set -euo pipefail

# Isolated coverage for the desktop distribution path: cask rendering, Homebrew's
# acceptance of the rendered cask, the inside-out signing order, the notarization
# and stapling invocations, artifact assembly, and every fail-closed branch.
#
# It reaches no Apple service, needs no Developer ID, installs nothing, and never
# touches /Applications or a published tap. Signing runs against the ad-hoc
# identity or a recording stub; notarization runs against a stub whose recorded
# invocation is the assertion.
#
# It does require Homebrew, and section 4 writes inside the local Homebrew
# prefix: it creates the throwaway tap
# `$(brew --repository)/Library/Taps/agentdeck-fixture`, loads the rendered casks
# out of it, and removes it again on the success path and through the `trap`. No
# other tap is read or written. That prerequisite reaches
# `make check-macos-distribution` and therefore `release-verify`; a missing
# `brew` fails the run rather than skipping section 4, because a silently
# skipped load check is indistinguishable from a passing one.

root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d "${TMPDIR:-/private/tmp}/agentdeck-macos-distribution.XXXXXX")
trap 'rm -rf "$temporary"' EXIT

cask_template="$root/packaging/homebrew/agentdeck-app.rb.tmpl"
dmg_sha=cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
zip_sha=dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd

checksums_for() {
  local tag=$1 file=$2
  printf '%s  AgentDeck_%s_universal.dmg\n%s  AgentDeck_%s_universal.zip\n' \
    "$dmg_sha" "$tag" "$zip_sha" "$tag" >"$file"
}

# 1. Stable cask.
stable_checksums="$temporary/stable-checksums.txt"
stable_cask="$temporary/agentdeck-app.rb"
checksums_for v1.2.3 "$stable_checksums"
bash "$root/scripts/render-homebrew-cask.sh" "$cask_template" v1.2.3 "$stable_checksums" "$stable_cask"
ruby -c "$stable_cask" >/dev/null
grep -F 'cask "agentdeck-app" do' "$stable_cask" >/dev/null
grep -F 'version "1.2.3"' "$stable_cask" >/dev/null
grep -F "sha256 \"$dmg_sha\"" "$stable_cask" >/dev/null
grep -F 'releases/download/v1.2.3/' "$stable_cask" >/dev/null
grep -F 'AgentDeck_v1.2.3_universal.dmg' "$stable_cask" >/dev/null
grep -F 'depends_on macos: :tahoe' "$stable_cask" >/dev/null
grep -F 'conflicts_with cask: ["agentdeck-app-rc"]' "$stable_cask" >/dev/null
# Homebrew accepts only casks in conflicts_with, so the formula exclusion is a
# preflight refusal and must name both CLI channels.
grep -F '["agentdeck", "agentdeck-rc"].each do |conflicting_formula|' "$stable_cask" >/dev/null
grep -F 'HOMEBREW_CELLAR/conflicting_formula' "$stable_cask" >/dev/null
grep -F 'app "AgentDeck.app"' "$stable_cask" >/dev/null
grep -F 'binary "#{appdir}/AgentDeck.app/Contents/Helpers/agentdeck"' "$stable_cask" >/dev/null
grep -F 'etc/bash_completion.d/agentdeck' "$stable_cask" >/dev/null
grep -F 'share/zsh/site-functions/_agentdeck' "$stable_cask" >/dev/null
grep -F 'share/fish/vendor_completions.d/agentdeck.fish' "$stable_cask" >/dev/null
grep -F 'brew uninstall agentdeck' "$stable_cask" >/dev/null
grep -F 'brew install --cask agentdeck-app' "$stable_cask" >/dev/null
grep -F 'Your AgentDeck state in ~/.agentdeck is untouched' "$stable_cask" >/dev/null
# The uninstall boundary is what the zap list must NOT contain.
if grep -F '.agentdeck"' "$stable_cask" | grep -F 'zap' >/dev/null; then
  echo "cask zap must not remove ~/.agentdeck" >&2
  exit 1
fi
if awk '/zap trash: \[/, /\]/' "$stable_cask" | grep -F '~/.agentdeck' >/dev/null; then
  echo "cask zap list must not contain ~/.agentdeck" >&2
  exit 1
fi
test "$(stat -f '%Lp' "$stable_cask")" = 644

# 2. RC cask is a separate token that excludes the stable one.
rc_checksums="$temporary/rc-checksums.txt"
rc_cask="$temporary/agentdeck-app-rc.rb"
checksums_for v1.2.3-rc.1 "$rc_checksums"
bash "$root/scripts/render-homebrew-cask.sh" "$cask_template" v1.2.3-rc.1 "$rc_checksums" "$rc_cask"
ruby -c "$rc_cask" >/dev/null
grep -F 'cask "agentdeck-app-rc" do' "$rc_cask" >/dev/null
grep -F 'version "1.2.3-rc.1"' "$rc_cask" >/dev/null
grep -F 'conflicts_with cask: ["agentdeck-app"]' "$rc_cask" >/dev/null
grep -F 'brew install --cask agentdeck-app-rc' "$rc_cask" >/dev/null

# 3. Rejected tags and checksum shapes.
if bash "$root/scripts/render-homebrew-cask.sh" \
     "$cask_template" v1.2.3-beta.1 "$rc_checksums" "$temporary/beta.rb" >/dev/null 2>&1; then
  echo "cask renderer accepted a non-RC prerelease tag" >&2
  exit 1
fi
printf '%s  AgentDeck_v1.2.3_universal.zip\n' "$zip_sha" >"$temporary/missing-dmg.txt"
if bash "$root/scripts/render-homebrew-cask.sh" \
     "$cask_template" v1.2.3 "$temporary/missing-dmg.txt" "$temporary/missing.rb" >/dev/null 2>&1; then
  echo "cask renderer accepted checksums without the DMG entry" >&2
  exit 1
fi
printf '%s  AgentDeck_v1.2.3_universal.dmg\n' "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC" >"$temporary/uppercase.txt"
if bash "$root/scripts/render-homebrew-cask.sh" \
     "$cask_template" v1.2.3 "$temporary/uppercase.txt" "$temporary/uppercase.rb" >/dev/null 2>&1; then
  echo "cask renderer accepted a non-lowercase checksum" >&2
  exit 1
fi

# 4. Homebrew's own verdict on the rendered cask. Every assertion above reads
# the file the renderer just wrote, so on its own section 1 restates the
# template; the defect it structurally cannot see is a stanza Homebrew rejects
# at load time, which makes the whole cask — app, binaries, caveats, zap —
# unreachable rather than merely wrong. Loading it through a throwaway tap is
# what turns that from an unknown into a result.
if ! command -v brew >/dev/null 2>&1; then
  echo "the cask load check requires Homebrew, which is a hard prerequisite of" >&2
  echo "make check-macos-distribution and therefore of release-verify; this check" >&2
  echo "fails rather than skipping, because Homebrew's verdict is the only thing" >&2
  echo "here that the rendered cask cannot restate about itself" >&2
  exit 1
fi
homebrew_taps="$(brew --repository)/Library/Taps"
fixture_root="$homebrew_taps/agentdeck-fixture"
fixture_tap="$fixture_root/homebrew-cask-fixture"
if [[ -e $fixture_root ]]; then
  echo "a previous run left $fixture_root behind; remove it before retrying" >&2
  exit 1
fi
trap 'rm -rf "$fixture_root" "$temporary"' EXIT
mkdir -p "$fixture_tap/Casks"

for pair in "agentdeck-app:$stable_cask" "agentdeck-app-rc:$rc_cask"; do
  token=${pair%%:*}
  rendered=${pair##*:}
  cp "$rendered" "$fixture_tap/Casks/$token.rb"
  load_log="$temporary/brew-info-$token.log"
  if ! env HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ANALYTICS=1 \
       brew info --cask "agentdeck-fixture/cask-fixture/$token" >"$load_log" 2>&1; then
    echo "Homebrew rejected the rendered $token cask:" >&2
    cat "$load_log" >&2
    exit 1
  fi
  # A deprecated stanza still loads, so its warning is the only signal it emits
  # and an unasserted warning is how a deprecation reaches a published tap.
  if grep -Eiq 'deprecat|^warning' "$load_log"; then
    echo "Homebrew warned while loading the $token cask:" >&2
    cat "$load_log" >&2
    exit 1
  fi
  # Every artifact class the cask declares must survive the load, because the
  # rejection this section exists to catch takes all of them down together.
  grep -F 'Required: macOS >= 26' "$load_log" >/dev/null
  grep -F 'AgentDeck.app (App)' "$load_log" >/dev/null
  grep -F 'AgentDeck.app/Contents/Helpers/agentdeck (Binary)' "$load_log" >/dev/null
  grep -F 'share/fish/vendor_completions.d/agentdeck.fish (Binary)' "$load_log" >/dev/null
  grep -F 'etc/bash_completion.d/agentdeck (Binary)' "$load_log" >/dev/null
  grep -F 'share/zsh/site-functions/_agentdeck (Binary)' "$load_log" >/dev/null
  grep -F 'brew install --cask ' "$load_log" >/dev/null
done
rm -rf "$fixture_root"
trap 'rm -rf "$temporary"' EXIT

# 5. A synthetic bundle drives the real packaging script. Its executables are
# real Mach-O binaries because entitlements only attach to real code.
make_bundle() {
  local bundle=$1 version=$2 build_number=${3:-1}
  local widget="$bundle/Contents/PlugIns/AgentDeckWidget.appex"
  rm -rf "$bundle"
  mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Helpers" "$widget/Contents/MacOS"
  printf 'int main(void) { return 0; }\n' >"$temporary/stub.c"
  cc -o "$bundle/Contents/MacOS/AgentDeck" "$temporary/stub.c"
  cc -o "$widget/Contents/MacOS/AgentDeckWidget" "$temporary/stub.c"
  cat >"$bundle/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.kitdine.agentdeck</string>
<key>CFBundleName</key><string>AgentDeck</string>
<key>CFBundleExecutable</key><string>AgentDeck</string>
<key>CFBundleShortVersionString</key><string>$version</string>
<key>CFBundleVersion</key><string>$build_number</string>
<key>CFBundlePackageType</key><string>APPL</string>
</dict></plist>
PLIST
  cat >"$widget/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.kitdine.agentdeck.widget</string>
<key>CFBundleName</key><string>AgentDeckWidget</string>
<key>CFBundleExecutable</key><string>AgentDeckWidget</string>
<key>CFBundleShortVersionString</key><string>$version</string>
<key>CFBundleVersion</key><string>$build_number</string>
<key>CFBundlePackageType</key><string>XPC!</string>
</dict></plist>
PLIST
  cat >"$bundle/Contents/Helpers/agentdeck" <<'HELPER'
#!/bin/sh
case "$*" in
"--format json version") printf '{"version":"@TAG@","commit":"0000000000000000000000000000000000000000"}\n' ;;
"completion bash") printf '# bash completion for agentdeck\n' ;;
"completion zsh") printf '#compdef agentdeck\n' ;;
"completion fish") printf '# fish completion for agentdeck\n' ;;
*) exit 64 ;;
esac
HELPER
  sed -i '' "s/@TAG@/v$version/" "$bundle/Contents/Helpers/agentdeck"
  chmod 0755 "$bundle/Contents/Helpers/agentdeck"
}

bundle="$temporary/AgentDeck.app"
package_dist="$temporary/package-dist"
make_bundle "$bundle" 1.2.3 37
mkdir -p "$package_dist"
AGENTDECK_SKIP_NOTARIZATION=1 \
  bash "$root/scripts/package-macos-app.sh" "$bundle" v1.2.3 "$package_dist" \
  >"$temporary/package.log" 2>&1

dmg="$package_dist/AgentDeck_v1.2.3_universal.dmg"
zip_archive="$package_dist/AgentDeck_v1.2.3_universal.zip"
package_checksums="$package_dist/AgentDeck_v1.2.3_checksums.txt"
test -f "$dmg"
test -f "$zip_archive"
test "$(plutil -extract CFBundleVersion raw -o - "$bundle/Contents/Info.plist")" = 37
test "$(plutil -extract CFBundleVersion raw -o - "$bundle/Contents/PlugIns/AgentDeckWidget.appex/Contents/Info.plist")" = 37
test "$(wc -l <"$package_checksums" | tr -d ' ')" -eq 2

mismatched_bundle="$temporary/AgentDeck-mismatched.app"
cp -R "$bundle" "$mismatched_bundle"
plutil -replace CFBundleVersion -string 38 \
  "$mismatched_bundle/Contents/PlugIns/AgentDeckWidget.appex/Contents/Info.plist"
if AGENTDECK_SKIP_NOTARIZATION=1 \
  bash "$root/scripts/package-macos-app.sh" \
    "$mismatched_bundle" v1.2.3 "$temporary/mismatched-dist" \
    >"$temporary/mismatched-build.log" 2>&1; then
  echo "packaging accepted mismatched App and Widget build numbers" >&2
  exit 1
fi
grep -F 'widget bundle build 38 does not match app build 37' \
  "$temporary/mismatched-build.log" >/dev/null
(
  cd "$package_dist"
  shasum -a 256 -c "$(basename "$package_checksums")" >/dev/null
)
test ! -e "$package_dist/AgentDeck.resolved.entitlements"
test ! -e "$package_dist/AgentDeckWidget.resolved.entitlements"

# The packaged bundle carries the completions the cask exposes.
for shell_file in agentdeck.bash agentdeck.zsh agentdeck.fish; do
  test -s "$bundle/Contents/Resources/completions/$shell_file"
  test "$(stat -f '%Lp' "$bundle/Contents/Resources/completions/$shell_file")" = 644
done

# The signature is real and the widget's entitlements carry the app group.
codesign --verify --deep --strict "$bundle"
codesign --display --entitlements - "$bundle/Contents/PlugIns/AgentDeckWidget.appex" 2>/dev/null |
  grep -F 'group.com.kitdine.agentdeck' >/dev/null
# Gatekeeper is assessed on every run; an ad-hoc bundle is correctly rejected,
# and the rejection being visible is the point.
grep -F ': rejected' "$temporary/package.log" >/dev/null

extracted="$temporary/extracted"
mkdir -p "$extracted"
ditto -x -k "$zip_archive" "$extracted"
test -x "$extracted/AgentDeck.app/Contents/Helpers/agentdeck"
test -s "$extracted/AgentDeck.app/Contents/Resources/completions/agentdeck.fish"
hdiutil imageinfo "$dmg" >"$temporary/imageinfo.txt"
grep -F 'Format: UDZO' "$temporary/imageinfo.txt" >/dev/null

# 6. Signing order, notarization, and stapling, observed through stubs. A real
# codesign cannot report the order it was called in; the order is the contract.
stub_bin="$temporary/stub-bin"
mkdir -p "$stub_bin"
cat >"$stub_bin/codesign" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf 'codesign %s\n' "$*" >>"$STUB_LOG"
if [[ ${1:-} == --display ]]; then
  printf 'group.com.kitdine.agentdeck\n'
fi
exit 0
STUB
cat >"$stub_bin/notarytool" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf 'notarytool %s\n' "$*" >>"$STUB_LOG"
exit 0
STUB
cat >"$stub_bin/stapler" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf 'stapler %s\n' "$*" >>"$STUB_LOG"
# A real staple amends the bundle. Recording that amendment is what lets the
# ZIP assertion below distinguish "stapled then archived" from the reverse,
# which no ordering claim about the log alone can establish.
if [[ ${1:-} == staple && ${2:-} == *.app ]]; then
  printf 'ticket\n' >"${2}/Contents/CodeResources.staple-marker"
fi
exit 0
STUB
cat >"$stub_bin/spctl" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf 'spctl %s\n' "$*" >>"$STUB_LOG"
printf 'accepted\n'
exit 0
STUB
chmod 0755 "$stub_bin"/*

stub_bundle="$temporary/StubAgentDeck.app"
stub_dist="$temporary/stub-dist"
make_bundle "$stub_bundle" 1.2.3
mkdir -p "$stub_dist"
stub_log="$temporary/stub.log"
stub_keychain="$temporary/agentdeck-signing.keychain-db"
: >"$stub_log"
: >"$stub_keychain"
STUB_LOG="$stub_log" \
  AGENTDECK_CODESIGN="$stub_bin/codesign" \
  AGENTDECK_NOTARY_TOOL="$stub_bin/notarytool" \
  AGENTDECK_STAPLER="$stub_bin/stapler" \
  AGENTDECK_SPCTL="$stub_bin/spctl" \
  AGENTDECK_SIGN_IDENTITY="Developer ID Application: Example (TEAMID)" \
  AGENTDECK_NOTARY_PROFILE=agentdeck-test \
  AGENTDECK_NOTARY_KEYCHAIN="$stub_keychain" \
  AGENTDECK_REQUIRE_GATEKEEPER=1 \
  bash "$root/scripts/package-macos-app.sh" "$stub_bundle" v1.2.3 "$stub_dist" >/dev/null 2>&1

signing_order=$(grep -c . "$stub_log")
test "$signing_order" -ge 6
helper_line=$(grep -n 'Helpers/agentdeck' "$stub_log" | head -1 | cut -d: -f1)
widget_line=$(grep -n 'AgentDeckWidget.appex' "$stub_log" | grep -v -- '--display' | head -1 | cut -d: -f1)
app_line=$(grep -n "codesign .*--entitlements .*AgentDeck.resolved.entitlements" "$stub_log" | head -1 | cut -d: -f1)
verify_line=$(grep -n -- '--verify --deep --strict' "$stub_log" | head -1 | cut -d: -f1)
test "$helper_line" -lt "$widget_line"
test "$widget_line" -lt "$app_line"
test "$app_line" -lt "$verify_line"
grep -F -- '--options runtime' "$stub_log" >/dev/null
grep -F -- '--timestamp' "$stub_log" >/dev/null
if grep -F -- '--timestamp=none' "$stub_log" >/dev/null; then
  echo "a real identity must not sign without a secure timestamp" >&2
  exit 1
fi
grep -F 'notarytool submit' "$stub_log" >/dev/null
# A profile stored into a non-default keychain is unreadable unless submit names
# the same keychain, so the two must be asserted as a pair rather than singly.
grep -F -- "--keychain-profile agentdeck-test --keychain $stub_keychain --wait" "$stub_log" >/dev/null
# Both artifacts are stapled: the DMG the cask points at, and the bundle the
# direct-download ZIP carries.
grep -E 'stapler staple .*/AgentDeck_v1\.2\.3_universal\.dmg$' "$stub_log" >/dev/null
grep -E 'stapler validate .*/AgentDeck_v1\.2\.3_universal\.dmg$' "$stub_log" >/dev/null
grep -E 'stapler staple .*/StubAgentDeck\.app$' "$stub_log" >/dev/null
grep -E 'stapler validate .*/StubAgentDeck\.app$' "$stub_log" >/dev/null
grep -F 'spctl --assess --type execute' "$stub_log" >/dev/null

# The ZIP is the direct-download artifact and carries the bundle unwrapped, so
# it must be assembled from the stapled bundle; one made first ships a ticketless
# app that fails first launch offline.
stub_extracted="$temporary/stub-extracted"
mkdir -p "$stub_extracted"
ditto -x -k "$stub_dist/AgentDeck_v1.2.3_universal.zip" "$stub_extracted"
if [[ ! -f "$stub_extracted/StubAgentDeck.app/Contents/CodeResources.staple-marker" ]]; then
  echo "the direct-download ZIP was assembled before the bundle was stapled" >&2
  exit 1
fi

# The notarization keychain is resolved before anything is signed, so a path
# that does not exist refuses the run instead of failing at the upload.
missing_keychain_bundle="$temporary/MissingKeychain.app"
make_bundle "$missing_keychain_bundle" 1.2.3
if AGENTDECK_CODESIGN="$stub_bin/codesign" \
     AGENTDECK_NOTARY_TOOL="$stub_bin/notarytool" \
     AGENTDECK_STAPLER="$stub_bin/stapler" \
     AGENTDECK_SPCTL="$stub_bin/spctl" \
     AGENTDECK_SIGN_IDENTITY="Developer ID Application: Example (TEAMID)" \
     AGENTDECK_NOTARY_PROFILE=agentdeck-test \
     AGENTDECK_NOTARY_KEYCHAIN="$temporary/absent.keychain-db" \
     STUB_LOG="$temporary/unused-stub.log" \
     bash "$root/scripts/package-macos-app.sh" \
     "$missing_keychain_bundle" v1.2.3 "$temporary/fail-keychain" >/dev/null 2>&1; then
  echo "packaging accepted a notarization keychain that does not exist" >&2
  exit 1
fi

# 7. Fail-closed branches. Each must refuse before producing an artifact.
fail_closed() {
  local name=$1
  shift
  local case_dist="$temporary/fail-$name"
  local case_bundle="$temporary/Fail$name.app"
  mkdir -p "$case_dist"
  make_bundle "$case_bundle" 1.2.3
  if env "$@" bash "$root/scripts/package-macos-app.sh" "$case_bundle" "${FAIL_TAG:-v1.2.3}" "$case_dist" >/dev/null 2>&1; then
    echo "packaging accepted $name" >&2
    exit 1
  fi
  if compgen -G "$case_dist/*.dmg" >/dev/null; then
    echo "packaging produced a DMG while failing $name" >&2
    exit 1
  fi
}

fail_closed missing-notary-credential AGENTDECK_SKIP_NOTARIZATION=0
fail_closed adhoc-notarization AGENTDECK_SKIP_NOTARIZATION=0 AGENTDECK_NOTARY_PROFILE=agentdeck-test
FAIL_TAG=v1.2.4 fail_closed version-mismatch AGENTDECK_SKIP_NOTARIZATION=1
FAIL_TAG=v1.2.3-beta.1 fail_closed invalid-tag AGENTDECK_SKIP_NOTARIZATION=1
unset FAIL_TAG

missing_widget="$temporary/MissingWidget.app"
make_bundle "$missing_widget" 1.2.3
rm -rf "$missing_widget/Contents/PlugIns"
if AGENTDECK_SKIP_NOTARIZATION=1 \
     bash "$root/scripts/package-macos-app.sh" "$missing_widget" v1.2.3 "$temporary/fail-widget" >/dev/null 2>&1; then
  echo "packaging accepted an app bundle without the widget extension" >&2
  exit 1
fi

# 8. The Cask tap pull request is a second, independent channel: it writes
# Casks/, never Formula/, and opens its own pull request.
tap_bin="$temporary/tap-bin"
mkdir -p "$tap_bin"
cat >"$tap_bin/gh" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
if [[ $1 == pr && $2 == list ]]; then
  printf '%s' "${TEST_PR_NUMBER:-}"
  exit 0
fi
if [[ $1 == pr && $2 == create ]]; then
  printf '%s\n' "$*" >>"$TEST_GH_LOG"
  exit 0
fi
echo "unexpected gh invocation: $*" >&2
exit 1
STUB
chmod 0755 "$tap_bin/gh"

run_cask_tap_case() (
  case_name=$1
  tag=$2
  expected_token=$3
  case_root="$temporary/cask-tap-$case_name"
  bare="$case_root/origin.git"
  seed="$case_root/seed"
  checkout="$case_root/checkout"
  rendered="$case_root/cask.rb"
  gh_log="$case_root/gh.log"

  mkdir -p "$case_root"
  git init --bare --quiet "$bare"
  git init --quiet "$seed"
  git -C "$seed" switch --create main >/dev/null
  git -C "$seed" config user.name "Tap Maintainer"
  git -C "$seed" config user.email "tap-maintainer@example.invalid"
  mkdir -p "$seed/Formula" "$seed/Casks"
  printf 'class Agentdeck < Formula\n  version "1.0.0"\nend\n' >"$seed/Formula/agentdeck.rb"
  git -C "$seed" add Formula/agentdeck.rb
  git -C "$seed" commit --quiet -m "initial formula"
  git -C "$seed" remote add origin "$bare"
  git -C "$seed" push --quiet --set-upstream origin main
  git --git-dir="$bare" symbolic-ref HEAD refs/heads/main

  case_checksums="$case_root/checksums.txt"
  checksums_for "$tag" "$case_checksums"
  bash "$root/scripts/render-homebrew-cask.sh" "$cask_template" "$tag" "$case_checksums" "$rendered"

  git clone --quiet "$bare" "$checkout"
  : >"$gh_log"
  PATH="$tap_bin:$PATH" TEST_GH_LOG="$gh_log" HOMEBREW_TAP_REPOSITORY=kitdine/homebrew-tap \
    bash "$root/scripts/update-homebrew-tap-pr.sh" "$checkout" "$rendered" "$tag" cask >/dev/null 2>&1

  branch="$expected_token-$tag"
  git --git-dir="$bare" show "refs/heads/$branch:Casks/$expected_token.rb" >"$case_root/remote.rb"
  cmp "$rendered" "$case_root/remote.rb"
  changed=$(git --git-dir="$bare" diff --name-only "refs/heads/main..refs/heads/$branch")
  test "$changed" = "Casks/$expected_token.rb"
  grep -F -- "--title $expected_token $tag" "$gh_log" >/dev/null
  grep -F -- "--head $branch" "$gh_log" >/dev/null
  grep -F 'CLI-only formula is unchanged' "$gh_log" >/dev/null || \
    grep -F 'stable cask and the CLI-only formula are unchanged' "$gh_log" >/dev/null
)

run_cask_tap_case stable v1.2.3 agentdeck-app
run_cask_tap_case rc v1.2.3-rc.1 agentdeck-app-rc

if PATH="$tap_bin:$PATH" TEST_GH_LOG="$temporary/unused.log" \
     bash "$root/scripts/update-homebrew-tap-pr.sh" \
     "$temporary/cask-tap-stable/checkout" "$stable_cask" v1.2.3 formulaish >/dev/null 2>&1; then
  echo "tap update accepted an unknown artifact kind" >&2
  exit 1
fi

# 9. The aggregate gate actually reaches these checks. A gate that is not in the
# command that runs it does not run, and that is indistinguishable from passing.
gate_plan="$temporary/release-verify-plan.txt"
make -n -C "$root" release-verify >"$gate_plan" 2>/dev/null
for expected in \
  scripts/check-widget-sandbox.sh \
  scripts/test-macos-distribution.sh \
  scripts/test-cask-migration.sh; do
  if ! grep -F "$expected" "$gate_plan" >/dev/null; then
    echo "release-verify does not reach $expected" >&2
    exit 1
  fi
done

printf 'macOS distribution packaging: PASS\n'
