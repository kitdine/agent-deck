#!/usr/bin/env swift

import Darwin
import Foundation

let arguments = Array(CommandLine.arguments.dropFirst())
let hostBundleIdentifier = "com.kitdine.agentdeck"
let installedHost = URL(fileURLWithPath: "/Applications/AgentDeck.app/Contents/MacOS/AgentDeck")

guard ProcessInfo.processInfo.environment["__CFBundleIdentifier"] == hostBundleIdentifier else {
	fputs("reload-widget-timelines: set __CFBundleIdentifier=\(hostBundleIdentifier) before launching Swift\n", stderr)
	exit(EX_CONFIG)
}

switch arguments {
case ["--check"]:
	print("reload-widget-timelines: WidgetKit hook available")
case [], ["--reload"]:
	guard FileManager.default.isExecutableFile(atPath: installedHost.path) else {
		fputs("reload-widget-timelines: installed AgentDeck host is unavailable\n", stderr)
		exit(EX_UNAVAILABLE)
	}
	let process = Process()
	process.executableURL = installedHost
	process.arguments = ["--reload-widget-timelines"]
	do {
		try process.run()
		process.waitUntilExit()
	} catch {
		fputs("reload-widget-timelines: launch installed host: \(error)\n", stderr)
		exit(EX_OSERR)
	}
	guard process.terminationReason == .exit, process.terminationStatus == 0 else {
		fputs("reload-widget-timelines: installed host reload failed\n", stderr)
		exit(EX_SOFTWARE)
	}
	print("reload-widget-timelines: requested reload for all AgentDeck timelines")
case ["--help"], ["-h"]:
	print("usage: reload-widget-timelines.swift [--check|--reload]")
default:
	fputs("reload-widget-timelines: unexpected arguments\n", stderr)
	exit(EX_USAGE)
}
