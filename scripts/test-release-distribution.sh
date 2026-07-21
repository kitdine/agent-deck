#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d /private/tmp/agentdeck-release-distribution.XXXXXX)
trap 'rm -rf "$temporary"' EXIT

origin="$temporary/origin.git"
repository="$temporary/repository"
notes="$temporary/notes.md"
extracted="$temporary/extracted.md"
git init --bare --quiet "$origin"
git init --quiet "$repository"
git -C "$repository" config user.name "AgentDeck Test"
git -C "$repository" config user.email "agentdeck-test@example.invalid"
printf 'fixture\n' >"$repository/fixture"
git -C "$repository" add fixture
git -C "$repository" commit --quiet -m "fixture commit"
git -C "$repository" remote add origin "$origin"
printf '## Features\n\n- Preserve markdown headings.\n\n## Compatibility\n\n- Test only.\n' >"$notes"

(
  cd "$repository"
  "$root/scripts/create-release-tag.sh" v1.2.3 "$notes" >/dev/null
)
test "$(git -C "$repository" cat-file -t refs/tags/v1.2.3)" = tag
git -C "$repository" push --quiet origin refs/tags/v1.2.3
commit=$(git -C "$repository" rev-parse refs/tags/v1.2.3^{})

# Reproduce actions/checkout's tag-event behavior: the public tag ref points
# directly at the peeled commit even though the remote ref remains annotated.
git -C "$repository" update-ref refs/tags/v1.2.3 "$commit"
test "$(git -C "$repository" cat-file -t refs/tags/v1.2.3)" = commit
(
  cd "$repository"
  "$root/scripts/release-notes-from-tag.sh" origin v1.2.3 "$commit" "$extracted"
)
cmp "$notes" "$extracted"
test ! -e "$repository/.git/refs/agentdeck-release-tags/v1.2.3"

git -C "$repository" tag v1.2.4
git -C "$repository" push --quiet origin refs/tags/v1.2.4
if (
  cd "$repository"
  "$root/scripts/release-notes-from-tag.sh" origin v1.2.4 "$commit" "$temporary/lightweight.md" >/dev/null 2>&1
); then
  echo "release-note extraction accepted a lightweight tag" >&2
  exit 1
fi

arm64_sha=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
amd64_sha=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
checksums="$temporary/checksums.txt"
formula="$temporary/agentdeck.rb"
printf '%s  %s\n%s  %s\n' \
  "$arm64_sha" agentdeck_v1.2.3_darwin_arm64.tar.gz \
  "$amd64_sha" agentdeck_v1.2.3_darwin_amd64.tar.gz >"$checksums"
"$root/scripts/render-homebrew-formula.sh" \
  "$root/packaging/homebrew/agentdeck.rb.tmpl" v1.2.3 "$checksums" "$formula"
ruby -c "$formula" >/dev/null
grep -F 'version "1.2.3"' "$formula" >/dev/null
grep -F "arm:   \"$arm64_sha\"" "$formula" >/dev/null
grep -F "intel: \"$amd64_sha\"" "$formula" >/dev/null
grep -F 'on_arch_conditional arm: "arm64", intel: "amd64"' "$formula" >/dev/null
grep -F 'shell_parameter_format: :cobra' "$formula" >/dev/null
grep -F 'shells:                 [:bash, :zsh, :fish]' "$formula" >/dev/null
grep -F 'assert_path_exists bash_completion/"agentdeck"' "$formula" >/dev/null
grep -F 'assert_path_exists zsh_completion/"_agentdeck"' "$formula" >/dev/null
grep -F 'assert_path_exists fish_completion/"agentdeck.fish"' "$formula" >/dev/null
test "$(stat -f '%Lp' "$formula")" = 644

ruby "$root/scripts/check-release-workflow.rb" "$root/.github/workflows/release.yml"

workflow_without_notes_helper="$temporary/release-without-notes-helper.yml"
sed 's#scripts/release-notes-from-tag\.sh#scripts/missing-release-notes-helper.sh#' \
  "$root/.github/workflows/release.yml" >"$workflow_without_notes_helper"
if ruby "$root/scripts/check-release-workflow.rb" "$workflow_without_notes_helper" >/dev/null 2>&1; then
  echo "workflow check accepted a missing release-note helper" >&2
  exit 1
fi

workflow_with_notes_from_tag="$temporary/release-with-notes-from-tag.yml"
sed 's/--notes-file/--notes-from-tag/' \
  "$root/.github/workflows/release.yml" >"$workflow_with_notes_from_tag"
if ruby "$root/scripts/check-release-workflow.rb" "$workflow_with_notes_from_tag" >/dev/null 2>&1; then
  echo "workflow check accepted --notes-from-tag" >&2
  exit 1
fi

workflow_without_tap_helper="$temporary/release-without-tap-helper.yml"
sed 's#scripts/update-homebrew-tap-pr\.sh#scripts/missing-tap-helper.sh#' \
  "$root/.github/workflows/release.yml" >"$workflow_without_tap_helper"
if ruby "$root/scripts/check-release-workflow.rb" "$workflow_without_tap_helper" >/dev/null 2>&1; then
  echo "workflow check accepted a missing tap PR helper" >&2
  exit 1
fi

workflow_without_stable_filter="$temporary/release-without-stable-filter.yml"
sed "s/ && !contains(github.ref_name, '-')//" \
  "$root/.github/workflows/release.yml" >"$workflow_without_stable_filter"
if ruby "$root/scripts/check-release-workflow.rb" "$workflow_without_stable_filter" >/dev/null 2>&1; then
  echo "workflow check accepted a missing stable-release filter" >&2
  exit 1
fi

fake_bin="$temporary/bin"
mkdir -p "$fake_bin"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  '' \
  'if [[ $1 == pr && $2 == list ]]; then' \
  '  printf '\''%s'\'' "${TEST_PR_NUMBER:-}"' \
  '  exit 0' \
  'fi' \
  'if [[ $1 == pr && $2 == create ]]; then' \
  '  printf '\''%s\n'\'' "$*" >>"$TEST_GH_LOG"' \
  '  exit 0' \
  'fi' \
  'echo "unexpected gh invocation: $*" >&2' \
  'exit 1' >"$fake_bin/gh"
chmod 0755 "$fake_bin/gh"

run_tap_case() (
  case_name=$1
  branch_formula=$2
  pr_number=$3
  expected_new_commits=$4
  expected_pr_creates=$5
  expected_result=${6:-success}
  case_root="$temporary/tap-$case_name"
  bare="$case_root/origin.git"
  seed="$case_root/seed"
  checkout="$case_root/checkout"
  desired="$case_root/desired.rb"
  gh_log="$case_root/gh.log"
  branch=agentdeck-v1.2.3

  mkdir -p "$case_root"
  git init --bare --quiet "$bare"
  git init --quiet "$seed"
  git -C "$seed" switch --create main >/dev/null
  git -C "$seed" config user.name "Tap Maintainer"
  git -C "$seed" config user.email "tap-maintainer@example.invalid"
  mkdir -p "$seed/Formula"
  printf 'class Agentdeck < Formula\n  version "1.0.0"\nend\n' >"$seed/Formula/agentdeck.rb"
  git -C "$seed" add Formula/agentdeck.rb
  git -C "$seed" commit --quiet -m "initial formula"
  git -C "$seed" remote add origin "$bare"
  git -C "$seed" push --quiet --set-upstream origin main
  git --git-dir="$bare" symbolic-ref HEAD refs/heads/main
  printf 'class Agentdeck < Formula\n  version "1.2.3"\nend\n' >"$desired"

  before_commits=0
  if [[ $branch_formula != none ]]; then
    git -C "$seed" switch --create "$branch" main >/dev/null
    git -C "$seed" config user.name "github-actions[bot]"
    git -C "$seed" config user.email "41898282+github-actions[bot]@users.noreply.github.com"
    if [[ $branch_formula == matching || $branch_formula == unsafe-matching ]]; then
      cp "$desired" "$seed/Formula/agentdeck.rb"
    else
      printf 'class Agentdeck < Formula\n  version "1.2.2"\nend\n' >"$seed/Formula/agentdeck.rb"
    fi
    if [[ $branch_formula == unsafe || $branch_formula == unsafe-matching ]]; then
      printf 'unrelated branch content\n' >"$seed/README.md"
    fi
    git -C "$seed" add --all
    git -C "$seed" commit --quiet -m "agentdeck v1.2.3"
    git -C "$seed" push --quiet --set-upstream origin "$branch"
    before_commits=$(git --git-dir="$bare" rev-list --count "refs/heads/main..refs/heads/$branch")
  fi

  git clone --quiet "$bare" "$checkout"
  : >"$gh_log"
  if PATH="$fake_bin:$PATH" \
       TEST_PR_NUMBER="$pr_number" \
       TEST_GH_LOG="$gh_log" \
       HOMEBREW_TAP_REPOSITORY=kitdine/homebrew-tap \
       bash "$root/scripts/update-homebrew-tap-pr.sh" "$checkout" "$desired" v1.2.3 >/dev/null 2>&1; then
    actual_result=success
  else
    actual_result=failure
  fi
  test "$actual_result" = "$expected_result"

  if [[ $expected_result == failure ]]; then
    after_commits=$(git --git-dir="$bare" rev-list --count "refs/heads/main..refs/heads/$branch")
    test "$after_commits" -eq "$before_commits"
    test ! -s "$gh_log"
    exit 0
  fi

  git --git-dir="$bare" show "refs/heads/$branch:Formula/agentdeck.rb" >"$case_root/remote.rb"
  cmp "$desired" "$case_root/remote.rb"
  after_commits=$(git --git-dir="$bare" rev-list --count "refs/heads/main..refs/heads/$branch")
  test "$((after_commits - before_commits))" -eq "$expected_new_commits"
  test "$(wc -l <"$gh_log" | tr -d ' ')" -eq "$expected_pr_creates"
)

run_tap_case no-branch none "" 1 1
run_tap_case matching-branch matching "" 0 1
run_tap_case stale-branch stale "" 1 1
run_tap_case matching-open-pr matching 42 0 0
run_tap_case stale-open-pr stale 42 1 0
run_tap_case unsafe-stale-branch unsafe "" 0 0 failure
run_tap_case unsafe-matching-open-pr unsafe-matching 42 0 0 failure
