#!/usr/bin/env bash
set -euo pipefail
trap 'status=$?; echo "check-desktop-refresh-integration.sh: assertion failed at line $LINENO (exit $status)" >&2' ERR

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
shared="$repo_root/apps/macos/AgentDeckShared"
widget="$repo_root/apps/macos/AgentDeckWidget"
app="$repo_root/apps/macos/AgentDeckApp"
design="$repo_root/docs/specs/cli-design.md"
manual="$repo_root/docs/specs/cli-manual.md"

if rg -n 'reloadAllTimelines' "$app" "$shared" "$widget"; then
  echo "legacy unconditional Widget reload remains" >&2
  exit 1
fi
if rg -n 'Data\(contentsOf:' "$widget"; then
  echo "Widget production retains an unbounded convenience read" >&2
  exit 1
fi
test "$(rg -l 'WidgetCenter\.shared\.reloadTimelines\(ofKind:' "$shared" | wc -l | tr -d ' ')" -eq 1
rg -Fq 'testTenTerminalCyclesHaveOneRequestEachAndNoOverlapReplay' "$repo_root/apps/macos/AgentDeckTests/DesktopRefreshSchedulerTests.swift"
rg -Fq 'static let fullRefreshInterval: TimeInterval = 60' "$shared/DesktopRefreshScheduler.swift"
rg -Fq 'static let evaluatorInterval: Duration = .seconds(30)' "$shared/DesktopRefreshScheduler.swift"
rg -Fq 'static let minimumRefresh: TimeInterval = 3 * 60' "$widget/WidgetTimeline.swift"
rg -Fq 'static let defaultRefresh: TimeInterval = 4 * 60' "$widget/WidgetTimeline.swift"
rg -Fq 'static let maximumRefresh: TimeInterval = 5 * 60' "$widget/WidgetTimeline.swift"
rg -Fq 'AppGroupSnapshotBytes.readBounded' "$widget/WidgetSnapshot.swift"
rg -Fq 'one-minute refresh interval' "$design"
rg -Uq 'fifteen\n  configurations in total' "$design"
rg -Fq '十五种配置' "$manual"
rg -Fq 'native acceptance gaps' "$design"
rg -Fq 'native acceptance' "$manual"
if rg -Fq 'macOS approves both host and Widget container access' "$design"; then
  echo "stable design overclaims installed Widget acceptance" >&2
  exit 1
fi
if rg -Fq '十五种配置均渲染真实数据' "$manual"; then
  echo "stable manual overclaims installed Widget rendering" >&2
  exit 1
fi

echo "desktop refresh integration contract: PASS"
