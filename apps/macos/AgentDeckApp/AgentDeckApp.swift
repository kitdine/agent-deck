import AppKit
import AgentDeckShared
import SwiftUI

@MainActor
private final class AgentDeckApplicationDelegate: NSObject, NSApplicationDelegate {
	let refreshCoordinator = DesktopRefreshCoordinator()

	func applicationDidFinishLaunching(_ notification: Notification) {
		refreshCoordinator.startInitialRefresh()
	}
}

@main
@available(macOS 26.0, *)
struct AgentDeckApp: App {
	@NSApplicationDelegateAdaptor(AgentDeckApplicationDelegate.self) private var appDelegate

	var body: some Scene {
		MenuBarExtra("AgentDeck", systemImage: "rectangle.stack") {
			VStack(alignment: .leading, spacing: 8) {
				Text("AgentDeck desktop foundation")
					.font(.headline)
				Text("Menu-bar summaries and controls arrive in the next task.")
					.foregroundStyle(.secondary)
				Divider()
				Button("Quit AgentDeck") {
					NSApplication.shared.terminate(nil)
				}
				.keyboardShortcut("q")
			}
			.padding()
			.environment(appDelegate.refreshCoordinator)
		}
		.menuBarExtraStyle(.window)
	}
}
