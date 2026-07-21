#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: update-homebrew-tap-pr.sh <tap-repository> <formula> <version-tag>" >&2
  exit 2
fi

tap_repository=$1
formula=$2
tag=$3
remote=origin
base_branch=main
repository=${HOMEBREW_TAP_REPOSITORY:-kitdine/homebrew-tap}
bot_name=github-actions[bot]
bot_email=41898282+github-actions[bot]@users.noreply.github.com

if [[ ! $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo "Homebrew tap updates require a stable semantic version tag: $tag" >&2
  exit 2
fi
if [[ ! -d $tap_repository/.git ]]; then
  echo "Homebrew tap checkout is not a Git repository: $tap_repository" >&2
  exit 2
fi
if [[ ! -f $formula || -L $formula ]]; then
  echo "rendered formula must be a regular file: $formula" >&2
  exit 2
fi

tap_repository=$(cd "$tap_repository" && pwd)
formula=$(cd "$(dirname "$formula")" && pwd)/$(basename "$formula")
branch="agentdeck-$tag"
formula_path=Formula/agentdeck.rb
remote_branch="refs/remotes/$remote/$branch"
temporary=$(mktemp "${TMPDIR:-/tmp}/agentdeck-tap-formula.XXXXXX")
trap 'rm -f "$temporary"' EXIT

cd "$tap_repository"
git config user.name "$bot_name"
git config user.email "$bot_email"

existing_pr=$(gh pr list \
  --repo "$repository" \
  --state open \
  --head "kitdine:$branch" \
  --json number \
  --jq '.[0].number // empty')

if [[ -z $existing_pr ]] && cmp -s "$formula" "$formula_path"; then
  echo "Homebrew formula already matches $tag"
  exit 0
fi

branch_exists=0
if git ls-remote --exit-code --heads "$remote" "$branch" >/dev/null; then
  branch_exists=1
  git fetch --force --no-tags "$remote" \
    "refs/heads/$branch:$remote_branch" >/dev/null

  changed_paths=$(git diff --name-only "$remote/$base_branch...$remote_branch")
  if [[ $changed_paths != "$formula_path" ]]; then
    echo "refusing to reuse $branch: it contains changes outside $formula_path" >&2
    exit 1
  fi
  branch_commits=$(git rev-list "$remote/$base_branch..$remote_branch")
  if [[ -z $branch_commits ]]; then
    echo "refusing to reuse $branch: it has no workflow-owned commits" >&2
    exit 1
  fi
  while IFS= read -r author_email; do
    if [[ $author_email != "$bot_email" ]]; then
      echo "refusing to reuse $branch: it contains a non-workflow commit" >&2
      exit 1
    fi
  done < <(git log --format=%ae "$remote/$base_branch..$remote_branch")
fi

formula_matches_remote_branch=0
if [[ $branch_exists -eq 1 ]] && \
   git show "$remote_branch:$formula_path" >"$temporary" 2>/dev/null && \
   cmp -s "$formula" "$temporary"; then
  formula_matches_remote_branch=1
fi

if [[ $branch_exists -eq 1 && $formula_matches_remote_branch -eq 0 ]]; then
  git switch --create "$branch" --track "$remote_branch" >/dev/null
  cp "$formula" "$formula_path"
  git add "$formula_path"
  if git diff --cached --quiet -- "$formula_path"; then
    echo "remote formula comparison changed during update" >&2
    exit 1
  fi
  git commit -m "agentdeck $tag" >/dev/null
  git push "$remote" "HEAD:refs/heads/$branch"
  formula_matches_remote_branch=1
fi

if [[ $branch_exists -eq 0 ]]; then
  if [[ -n $existing_pr ]]; then
    echo "Homebrew formula pull request #$existing_pr has no remote branch $branch" >&2
    exit 1
  fi
  git switch --create "$branch" >/dev/null
  cp "$formula" "$formula_path"
  git add "$formula_path"
  git commit -m "agentdeck $tag" >/dev/null
  git push --set-upstream "$remote" "$branch"
  branch_exists=1
  formula_matches_remote_branch=1
fi

if [[ $formula_matches_remote_branch -ne 1 ]]; then
  echo "remote Homebrew formula does not match the rendered formula for $tag" >&2
  exit 1
fi
if [[ -n $existing_pr ]]; then
  echo "Homebrew formula pull request is current: #$existing_pr"
  exit 0
fi

gh pr create \
  --repo "$repository" \
  --base "$base_branch" \
  --head "$branch" \
  --title "agentdeck $tag" \
  --body "Update AgentDeck to $tag and install bash, zsh, and fish completions."
