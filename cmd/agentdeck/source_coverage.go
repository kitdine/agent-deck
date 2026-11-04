package main

func sourceCoverageWarning(state string) string {
	switch state {
	case "complete":
		return ""
	case "partial":
		return "source_coverage_partial"
	case "update_pending":
		return "source_update_pending"
	default:
		return "source_coverage_unknown"
	}
}
