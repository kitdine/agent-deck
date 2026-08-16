# AgentDeck macOS foundation

`AgentDeck.xcodeproj` is the canonical Apple project. It builds a macOS 26
menu-bar host that imports `AgentDeckShared` and embeds only the same-build
`agentdeck` helper at `AgentDeck.app/Contents/Helpers/agentdeck`.

The local identifiers are deliberately non-secret defaults:

- App: `com.kitdine.agentdeck`
- App Group: `group.com.kitdine.agentdeck`
- Reserved Widget extension: `com.kitdine.agentdeck.widget`

Release signing, a Developer Team, notarization credentials, and release
configuration are intentionally absent. They belong to the distribution task.

Use the focused shared-layer tests without reading any real AgentDeck or client
state. With full Xcode this runs the XCTest target; Command Line Tools run the
same fixture cases through the Foundation-only verifier:

```bash
make test-macos-app
```

An unsigned local application build requires a full Xcode installation. It
builds a local Go helper, disables code signing, and verifies that the result
contains that helper:

```bash
make build-macos-app
```

The shared types decode the canonical synthetic fixtures in
`desktop/fixtures/v1`; the App Group projection deliberately omits session IDs,
project names, models, timestamps, recovery commands, and all credentials.
`DesktopLogger` emits only fixed classifications, never helper output or
snapshot content.
