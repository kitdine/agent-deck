import Foundation

// Every user-visible string in the app target resolves through this file. The
// key inventory below is what the localization test iterates: a key that is not
// listed is a key nobody proves resolves in both shipped languages.
enum DesktopCopy {
	// Surface and qualifier copy.
	static let loading = "Loading…"
	static let offline = "Cannot reach the AgentDeck helper"
	static let failing = "Data could not be read"
	static let partial = "Some data unavailable"
	static let emptyToday = "No local activity today"
	static let emptySnapshot = "No activity in this snapshot"
	static let freshnessUpdated = "Updated %@"
	static let freshnessLastUpdated = "Last updated %@"
	static let retry = "Retry"
	static let refreshNow = "Refresh now"
	static let refreshTimedOut = "Refresh timed out; showing the previous snapshot"
	static let appName = "AgentDeck"
	static let badgedOffline = "AgentDeck — offline"
	static let badgedFailing = "AgentDeck — data could not be read"
	static let qualifierList = "State: %@"

	// Filters and hero.
	static let clientFilter = "Client"
	static let periodFilter = "Period"
	static let clientAll = "All"
	static let periodToday = "Today"
	static let period7d = "7D"
	static let period30d = "30D"
	static let costIncomplete = "Cost incomplete · %lld unpriced"
	static let costIncompleteAttribution = "Cost incomplete · attribution unavailable"
	static let heroCounts = "%1$lld events · %2$lld sessions · %3$lld projects"

	// Panels.
	static let panelUsage = "Usage"
	static let panelBreakdown = "Breakdown"
	static let panelAttribution = "Attribution"
	static let panelSessions = "Sessions"
	static let panelUnavailableMark = "%@ · data unavailable"
	static let sectionUnavailable = "Section could not be read this refresh"

	static let trendTitle = "Trend"
	static let trendWindow = "Last %lld days"
	static let trendHours = "24 hours"
	static let trendEvents = "%lld events"
	static let trendNow = "Now"
	static let chipAveragePerDay = "AVG / DAY"
	static let chipPeak = "PEAK"
	static let chipCacheHit = "CACHE HIT"
	static let chipPriciestHour = "PRICIEST HOUR"
	static let chipEvents = "EVENTS"
	static let notCapturedYet = "Not captured yet"

	static let modelsTitle = "Models"
	static let modelsEmpty = "No models in this period"
	static let tokenMixTitle = "Token mix"
	static let tokenMixNote = "cache write is billed"
	static let tokenInput = "Input"
	static let tokenOutput = "Output"
	static let tokenCacheRead = "Cache read"
	static let tokenCacheWrite = "Cache write"
	static let perClientTitle = "Client subtotals"

	static let qualityTitle = "Attribution quality"
	static let qualityDeterminable = "Determinable"
	static let qualityInferred = "Inferred"
	static let qualityUnattributed = "Unattributed"
	static let qualityAllProviders = "All providers"
	static let qualityByProvider = "By provider"
	static let pricingTitle = "Pricing coverage"
	static let pricingPriced = "%1$lld / %2$lld priced"
	static let pricingUnpriced = "Unpriced: %@"

	static let sessionsCount = "Session count"
	static let sessionsAverage = "Avg length"
	static let sessionsTotalTime = "Total time"
	static let sessionsMedian = "Median length"
	static let sessionsProjects = "Projects"
	static let sessionsProjectRows = "By project"
	static let sessionsRecent = "Recent sessions"
	static let sessionsEmpty = "No sessions in this period"
	static let sessionsProjectUnnamed = "No project"
	static let sessionsProjectCount = "%lld sessions"
	static let sessionsSignals = "Work signals"
	static let sessionsActivity = "Activity"
	static let sessionsWorkflow = "Workflow"
	static let sessionsTooling = "Tooling"
	static let sessionsPendingHint = "These fields are not in the snapshot yet"
	static let sessionsActivityCoding = "Coding"
	static let sessionsActivityDebugging = "Debugging"
	static let sessionsActivityConversation = "Conversation"
	static let sessionsActivityDelegation = "Delegation"
	static let sessionsFirstEdit = "First edit"
	static let sessionsFilesTouched = "Files touched"
	static let sessionsRetries = "Rework"
	static let sessionsRetriesNote = "edit, verify, edit again"
	static let sessionsEditsPerSession = "Edits per session"
	static let sessionsMetricMedian = "median"
	static let sessionsTopFile = "Most touched"
	static let sessionsToolCalls = "Tool calls"
	static let sessionsTopMCPServer = "Top MCP server"
	static let sessionsToolBash = "Bash"
	static let sessionsToolRead = "Read"
	static let sessionsToolEdit = "Edit"
	static let sessionsToolMCP = "MCP"
	static let sessionsToolOther = "Other"
	static let sessionsSubFeature = "Feature"
	static let sessionsSubRefactoring = "Refactoring"
	static let sessionsSubTesting = "Testing"
	static let sessionsSubMaintenance = "Maintenance"
	static let sessionsSubInvestigation = "Investigation"
	static let sessionsSubRepair = "Repair"
	static let sessionsSubExploration = "Exploration"
	static let sessionsSubBrainstorming = "Brainstorming"
	static let sessionsSubPlanning = "Planning"
	static let sessionsSubagent = "Subagent"
	static let sessionsSubWorkflow = "Skill / workflow"

	// Rhythm block.
	static let rhythmTitle = "Rhythm"
	static let rhythmScope = "Last 30 days · not affected by the filters above"
	static let rhythmActive = "ACTIVE DAYS"
	static let rhythmBusiest = "BUSIEST DAY"
	static let rhythmQuietest = "QUIETEST DAY"
	static let rhythmPeakWindow = "PEAK WINDOW"
	static let rhythmHourOfWeek = "Hour of week"
	static let rhythmActiveNote = "days with tokens"
	static let rhythmBusiestNote = "most tokens"
	static let rhythmQuietestNote = "fewest tokens"
	static let rhythmPeakNote = "%@ · most tokens"
	static let rhythmActiveValue = "%1$lld / %2$lld"
	static let rhythmCell = "%1$@ %2$@"
	static let calendarTitle = "Last 90 days"

	// Notice strip and health detail.
	static let healthNotice = "%lld checks not passing"
	static let healthTitle = "Health"
	static let healthBack = "Back"
	static let healthSource = "These checks come from agentdeck doctor."
	static let healthStatusOK = "OK"
	static let healthStatusWarning = "Warning"
	static let healthStatusFailed = "Failed"
	static let healthCopyRecovery = "Copy recovery command"
	static let healthCopied = "Copied"
	static let noticeMore = "and %lld more"

	// Warning codes.
	static let warningProviderUnavailable = "Provider state could not be read"
	static let warningProviderCandidatesUnavailable = "Provider switch options could not be read"
	static let warningUsageUnavailable = "Usage data could not be read"
	static let warningSessionsUnavailable = "Session data could not be read"
	static let warningHealthUnavailable = "Health checks could not be run"
	static let warningStateCloseFailed = "AgentDeck state did not close cleanly"
	static let warningSessionsCloseFailed = "The session index did not close cleanly"
	static let warningUnknown = "Unrecognized warning · %@"

	// Footer and provider switching.
	static let footerProviders = "Providers"
	static let footerProviderUnavailable = "Provider unavailable"
	static let switchingUnavailable = "Switching unavailable"
	static let switchNoOptions = "No available switches"
	static let switchMenuTitle = "Switch provider"
	static let switchNow = "NOW"
	static let switchReady = "Available"
	static let switchChooseTarget = "%lld targets"
	static let switchDirect = "direct"
	static let switchWrapper = "wrapper"
	static let switchConfirmDirect = "Switch %1$@ to %2$@, directly?"
	static let switchConfirmWrapper = "Switch %1$@ to %2$@, through the wrapper?"
	static let switchConfirmCredentialDirect = "Switch %1$@ to %2$@ using credential “%3$@”, directly?"
	static let switchConfirmCredentialWrapper = "Switch %1$@ to %2$@ using credential “%3$@”, through the wrapper?"
	static let switchConfirm = "Switch"
	static let switchCancel = "Cancel"
	static let switchDismiss = "Dismiss"
	static let switchInFlight = "Switching…"
	static let switchOptionBlocked = "Switch in progress"
	static let switchFailed = "Switch failed · %@"
	static let switchFailedDetail = "The switch did not complete. AgentDeck state is unchanged as far as the helper reported."
	static let switchIndeterminate = "The switch result could not be confirmed"
	static let switchIndeterminateDetail = "AgentDeck is refreshing to find out what the current route is."
	static let switchSucceeded = "Switched %1$@ to %2$@"
	static let reasonCredentialMissing = "No credential available"
	static let reasonCredentialClientMismatch = "Credential is not bound to this client"
	static let reasonWrapperNotConfigured = "No wrapper configured"
	static let reasonAlreadySelected = "Already selected"
	static let reasonUnknown = "Unavailable · %@"

	// Menu-bar item menu.
	static let menuShows = "Menu bar shows"
	static let menuSettings = "Settings…"
	static let menuAbout = "About AgentDeck"
	static let menuQuit = "Quit AgentDeck"

	// Settings window.
	static let settingsTitle = "AgentDeck Settings"
	static let settingsGroupGeneral = "General"
	static let settingsGroupMenuBar = "Menu bar"
	static let settingsLoginItem = "Launch at login"
	static let settingsLoginItemNote = "Registered as a system login item; no background daemon is installed"
	static let settingsLoginItemRefused = "Could not change the login item"
	static let settingsLoginItemApproval = "Waiting for approval in System Settings"
	static let settingsPeriodicRefresh = "Periodic refresh"
	static let settingsPeriodicRefreshNote =
		"Refreshes at the time the snapshot suggests; when off, only opening the panel or refreshing manually updates it"
	static let settingsMenuBarValue = "Shows"
	static let settingsMenuBarValueNote = "Switch to icon only when sharing your screen"
	static let settingsMenuBarValueCost = "Cost"
	static let settingsMenuBarValueTokens = "Tokens"
	static let settingsMenuBarValueIcon = "Icon only"
	static let settingsMenuBarScope = "Scope"
	static let settingsMenuBarScopeNote =
		"When following the panel, picking Codex there also narrows the menu bar to Codex"
	static let settingsMenuBarScopeAll = "All clients"
	static let settingsMenuBarScopeFollow = "Follow panel filter"

	/// The inventory the localization test walks. Every key above appears here.
	static let allKeys: [String] = [
		loading, offline, failing, partial, emptyToday, emptySnapshot,
		freshnessUpdated, freshnessLastUpdated, retry, refreshNow, refreshTimedOut, appName,
		badgedOffline, badgedFailing, qualifierList,
		clientFilter, periodFilter, clientAll, periodToday, period7d, period30d,
		costIncomplete, costIncompleteAttribution, heroCounts,
		panelUsage, panelBreakdown, panelAttribution, panelSessions,
		panelUnavailableMark, sectionUnavailable,
		trendTitle, trendWindow, trendHours, trendEvents, trendNow,
		chipAveragePerDay, chipPeak, chipCacheHit,
		chipPriciestHour, chipEvents, notCapturedYet,
		modelsTitle, modelsEmpty, tokenMixTitle, tokenMixNote, tokenInput,
		tokenOutput, tokenCacheRead, tokenCacheWrite, perClientTitle,
		qualityTitle, qualityDeterminable, qualityInferred, qualityUnattributed,
		qualityAllProviders, qualityByProvider, pricingTitle, pricingPriced,
		pricingUnpriced,
		sessionsCount, sessionsAverage, sessionsTotalTime, sessionsMedian, sessionsProjects,
		sessionsProjectRows, sessionsRecent, sessionsEmpty,
		sessionsProjectUnnamed, sessionsProjectCount, sessionsSignals,
		sessionsActivity, sessionsWorkflow, sessionsTooling, sessionsPendingHint,
		sessionsActivityCoding, sessionsActivityDebugging,
		sessionsActivityConversation, sessionsActivityDelegation,
		sessionsFirstEdit, sessionsFilesTouched, sessionsRetries,
		sessionsRetriesNote, sessionsEditsPerSession, sessionsMetricMedian,
		sessionsTopFile, sessionsToolCalls, sessionsTopMCPServer,
		sessionsToolBash, sessionsToolRead, sessionsToolEdit, sessionsToolMCP,
		sessionsToolOther, sessionsSubFeature, sessionsSubRefactoring,
		sessionsSubTesting, sessionsSubMaintenance, sessionsSubInvestigation,
		sessionsSubRepair, sessionsSubExploration, sessionsSubBrainstorming,
		sessionsSubPlanning, sessionsSubagent, sessionsSubWorkflow,
		rhythmTitle, rhythmScope, rhythmActive, rhythmBusiest, rhythmQuietest,
		rhythmPeakWindow, rhythmHourOfWeek, rhythmActiveNote, rhythmBusiestNote,
		rhythmQuietestNote, rhythmPeakNote, rhythmActiveValue, rhythmCell, calendarTitle,
		healthNotice, healthTitle, healthBack, healthSource, healthStatusOK,
		healthStatusWarning, healthStatusFailed, healthCopyRecovery, healthCopied,
		noticeMore,
		warningProviderUnavailable, warningProviderCandidatesUnavailable,
		warningUsageUnavailable, warningSessionsUnavailable,
		warningHealthUnavailable, warningStateCloseFailed,
		warningSessionsCloseFailed, warningUnknown,
		footerProviders, footerProviderUnavailable, switchingUnavailable,
		switchNoOptions, switchMenuTitle, switchNow, switchReady,
		switchChooseTarget, switchDirect, switchWrapper,
		switchConfirmDirect, switchConfirmWrapper, switchConfirmCredentialDirect,
		switchConfirmCredentialWrapper, switchConfirm, switchCancel,
		switchDismiss, switchInFlight, switchOptionBlocked, switchFailed,
		switchFailedDetail, switchIndeterminate, switchIndeterminateDetail,
		switchSucceeded, reasonCredentialMissing, reasonCredentialClientMismatch,
		reasonWrapperNotConfigured, reasonAlreadySelected, reasonUnknown,
		menuShows, menuSettings, menuAbout, menuQuit,
		settingsTitle, settingsGroupGeneral, settingsGroupMenuBar,
		settingsLoginItem, settingsLoginItemNote, settingsLoginItemRefused,
		settingsLoginItemApproval, settingsPeriodicRefresh,
		settingsPeriodicRefreshNote, settingsMenuBarValue,
		settingsMenuBarValueNote, settingsMenuBarValueCost,
		settingsMenuBarValueTokens, settingsMenuBarValueIcon,
		settingsMenuBarScope, settingsMenuBarScopeNote, settingsMenuBarScopeAll,
		settingsMenuBarScopeFollow,
	]
}

/// The locale the surface formats in. Production is the viewer's locale; the
/// acceptance harness pins it so the manual localization checklist can address
/// one language at a time.
enum DesktopLocale {
	static var override: String? {
		#if DEBUG
		let identifier = ProcessInfo.processInfo.environment["AGENTDECK_TEST_LOCALE"]
		return identifier == "en" || identifier == "zh-Hans" ? identifier : nil
		#else
		return nil
		#endif
	}

	static var current: Locale {
		guard let override else { return .current }
		return Locale(identifier: override)
	}

	static var bundle: Bundle {
		guard let override,
			let path = Bundle.main.path(forResource: override, ofType: "lproj"),
			let localized = Bundle(path: path)
		else {
			return .main
		}
		return localized
	}
}

/// Resolves one catalog key in the active bundle. The key doubles as the `en`
/// source string, so a missing translation is observable: the lookup returns the
/// key itself, which is what the localization test asserts never happens.
func t(_ key: String) -> String {
	DesktopLocale.bundle.localizedString(forKey: key, value: key, table: nil)
}

func t(_ key: String, _ arguments: any CVarArg...) -> String {
	String(format: t(key), locale: DesktopLocale.current, arguments: arguments)
}

/// Locale-aware presentation of the decimal strings and counts the wire carries.
/// Cost strings arrive as decimal text and are formatted without changing the
/// value; an inexact total keeps the `≈` the hero explains underneath.
enum DesktopFormat {
	static func cost(_ raw: String?, known: String, approximate: Bool) -> String {
		let value = raw ?? known
		guard let decimal = Decimal(string: value, locale: Locale(identifier: "en_US_POSIX")) else {
			return value
		}
		let rendered = decimal.formatted(.currency(code: "USD").locale(DesktopLocale.current))
		return approximate ? "≈\(rendered)" : rendered
	}

	static func tokens(_ value: Int64) -> String {
		value.formatted(.number.notation(.compactName).locale(DesktopLocale.current))
	}

	static func count(_ value: Int64) -> String {
		value.formatted(.number.locale(DesktopLocale.current))
	}

	static func percent(_ raw: String?) -> String {
		guard let raw, let value = Double(raw) else { return "—" }
		return (value / 100).formatted(.percent.precision(.fractionLength(0 ... 1)).locale(DesktopLocale.current))
	}

	static func workSignalPercent(_ value: Double?) -> String {
		guard let value else { return "—" }
		return (value / 100).formatted(.percent.precision(.fractionLength(0 ... 1)).locale(DesktopLocale.current))
	}

	static func workSignalCost(_ value: Double?) -> String {
		guard let value else { return "—" }
		return value.formatted(.currency(code: "USD").precision(.fractionLength(2)).locale(DesktopLocale.current))
	}

	static func workSignalCount(_ value: Int?) -> String {
		guard let value else { return "—" }
		return value.formatted(.number.locale(DesktopLocale.current))
	}

	static func workSignalDecimal(_ value: Double?) -> String {
		guard let value else { return "—" }
		return value.formatted(.number.precision(.fractionLength(0 ... 1)).locale(DesktopLocale.current))
	}

	static func workSignalDuration(_ seconds: Int?) -> String {
		guard let seconds else { return "—" }
		return Duration.seconds(max(0, seconds)).formatted(
			.units(allowed: [.hours, .minutes], width: .narrow).locale(DesktopLocale.current)
		)
	}

	static func decimalCompact(_ raw: String) -> String {
		guard let value = Double(raw) else { return raw }
		return value.formatted(.number.notation(.compactName).precision(.fractionLength(0 ... 1)).locale(DesktopLocale.current))
	}

	static func duration(_ seconds: Int64) -> String {
		guard seconds > 0 else { return "—" }
		return Duration.seconds(seconds).formatted(
			.units(allowed: [.hours, .minutes], width: .narrow).locale(DesktopLocale.current)
		)
	}

	static func day(_ value: String) -> String {
		guard let date = timestamp(value) else { return value }
		return date.formatted(.dateTime.weekday(.abbreviated).month(.abbreviated).day().locale(DesktopLocale.current))
	}

	static func compactDay(_ value: String) -> String {
		guard let date = timestamp(value) else { return value }
		return date.formatted(.dateTime.month(.abbreviated).day().locale(DesktopLocale.current))
	}

	static func relative(_ value: String, now: Date = Date()) -> String {
		guard let date = timestamp(value) else { return value }
		return date.formatted(.relative(presentation: .named).locale(DesktopLocale.current))
	}

	/// The wire indexes weekdays from Monday, as the rhythm producer does.
	/// Gregorian symbols start at Sunday, so the index is rotated rather than
	/// the weekday name being translated in the catalog.
	static func weekday(mondayBased index: Int) -> String {
		var calendar = Calendar(identifier: .gregorian)
		calendar.locale = DesktopLocale.current
		let symbols = calendar.shortWeekdaySymbols
		guard index >= 0, index < 7, symbols.count == 7 else { return "—" }
		return symbols[(index + 1) % 7]
	}

	/// `busiest_day` and `quietest_day` arrive as stable lowercase tokens.
	static func weekdayName(_ token: String) -> String {
		let order = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"]
		guard let index = order.firstIndex(of: token.lowercased()) else { return "—" }
		return weekday(mondayBased: index)
	}

	static func hourWindow(_ hour: Int) -> String {
		var calendar = Calendar(identifier: .gregorian)
		calendar.locale = DesktopLocale.current
		let start = calendar.date(from: DateComponents(year: 2000, month: 1, day: 1, hour: hour)) ?? Date()
		let end = calendar.date(byAdding: .hour, value: 1, to: start) ?? start
		let style = Date.FormatStyle.dateTime.hour().minute().locale(DesktopLocale.current)
		return "\(start.formatted(style))–\(end.formatted(style))"
	}

	static func hour(_ hour: Int) -> String {
		var calendar = Calendar(identifier: .gregorian)
		calendar.locale = DesktopLocale.current
		let start = calendar.date(from: DateComponents(year: 2000, month: 1, day: 1, hour: hour)) ?? Date()
		return start.formatted(.dateTime.hour().locale(DesktopLocale.current))
	}

	static func timestamp(_ value: String) -> Date? {
		let fractional = ISO8601DateFormatter()
		fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
		if let parsed = fractional.date(from: value) { return parsed }
		let basic = ISO8601DateFormatter()
		basic.formatOptions = [.withInternetDateTime]
		if let parsed = basic.date(from: value) { return parsed }
		let day = DateFormatter()
		day.locale = Locale(identifier: "en_US_POSIX")
		day.dateFormat = "yyyy-MM-dd"
		return day.date(from: value)
	}
}
