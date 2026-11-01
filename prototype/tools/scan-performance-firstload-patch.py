#!/usr/bin/env python3
"""Install the cold-only day-hint pilot in a private exported HEAD copy.

This is source priority feasibility, not a product flag or complete-day proof.
The normal (flag-off) scan and the priority pilot share one compiled executable.
"""
from pathlib import Path
import re
import sys

root = Path(sys.argv[1]).resolve()
if not re.fullmatch(r'/private/tmp/agentdeck-scan-performance-firstload\.[A-Za-z0-9]+', str(root)):
    raise SystemExit('Private exported firstload root required')
p = root / 'repo/internal/ingest/ingest.go'
text = p.read_text()
if 'AGENTDECK_FIRSTLOAD_DAY_HINTS' in text:
    raise SystemExit('Experiment already installed; do not patch twice')
if '"time"' not in text:
    text = text.replace('import (', 'import (\n\t"time"', 1)
marker = '\tsort.Slice(sources, func(i, j int) bool {'
assert text.count(marker) == 1
block = '''\t// Private experiment: cold-state priority hints only, not a complete inventory.
\t// Never enable against an existing database: omission could imply removal.
\tif os.Getenv("AGENTDECK_FIRSTLOAD_DAY_HINTS") == "1" {
\t\tnow := time.Now()
\t\tstart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
\t\tend := start.AddDate(0, 0, 1)
\t\tdays := []string{start.UTC().Format("2006/01/02"), end.Add(-time.Nanosecond).UTC().Format("2006/01/02")}
\t\tselected := make([]Source, 0)
\t\tfor _, source := range sources {
\t\t\thint := source.ModifiedAt >= start.UnixNano()
\t\t\tif source.Client == "codex" {
\t\t\t\tpath := filepath.ToSlash(source.Path)
\t\t\t\tfor _, day := range days {
\t\t\t\t\thint = hint || strings.Contains(path, "/sessions/"+day+"/") || strings.Contains(path, "/archived_sessions/"+day+"/")
\t\t\t\t}
\t\t\t}
\t\t\tif hint { selected = append(selected, source) }
\t\t}
\t\tsources = selected
\t}
'''
p.write_text(text.replace(marker, block + marker))
