package activity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// File is the only retained representation of a path observed in tool input.
type File struct {
	PathDigest string
	BaseName   string
	Wrote      bool
}

func classifyCodexTool(item map[string]any, itemKind, tool, machineIdentity string) (string, string, []File, string, bool) {
	if itemKind == "mcp_tool_call" {
		return "mcp", firstSafe(item["server"], item["server_name"], item["mcp_server"]), nil, "", false
	}
	arguments, raw := toolArguments(item)
	input := firstRaw(item["input"])
	switch tool {
	case "apply_patch":
		patch := firstRaw(arguments["patch"], arguments["input"], input, raw)
		return "edit", "", collectFiles(extractPatchPaths(patch), machineIdentity), "", false
	case "exec_command":
		cmd := firstRaw(arguments["cmd"])
		workdir := firstRaw(arguments["workdir"])
		kind, files, hint := classifyShellCommands([]string{cmd}, workdir, machineIdentity)
		return kind, "", files, hint, kind == "read"
	case "exec":
		payload := firstRaw(arguments["code"], arguments["source"], arguments["script"], input, raw)
		commands := execCommandLiterals(payload)
		kind, files, hint := classifyShellCommands(commands, firstRaw(arguments["workdir"]), machineIdentity)
		return kind, "", files, hint, kind == "read"
	case "js", "write_stdin":
		return "bash", "", nil, "", false
	case "wait", "wait_agent", "spawn_agent", "list_agents", "interrupt_agent", "send_message", "followup_task", "update_plan", "view_image":
		return "other", "", nil, "", false
	default:
		return "other", "", nil, "", false
	}
}

func classifyClaudeTool(item map[string]any, tool, machineIdentity string) (string, string, []File, string, bool) {
	input, _ := item["input"].(map[string]any)
	if input == nil {
		input = map[string]any{}
	}
	if strings.HasPrefix(tool, "mcp__") {
		parts := strings.SplitN(strings.TrimPrefix(tool, "mcp__"), "__", 2)
		return "mcp", parts[0], nil, "", false
	}
	switch tool {
	case "Edit", "Write", "NotebookEdit":
		path := firstRaw(input["file_path"], input["path"], input["notebook_path"])
		return "edit", "", collectFiles([]pathAccess{{path: path, wrote: true}}, machineIdentity), "", false
	case "Read", "Grep", "Glob":
		path := firstRaw(input["file_path"], input["path"])
		return "read", "", collectFiles([]pathAccess{{path: path}}, machineIdentity), "", false
	case "Bash":
		kind, files, hint := classifyShellCommands([]string{firstRaw(input["command"])}, "", machineIdentity)
		return "bash", "", files, hint, kind == "read"
	default:
		return "other", "", nil, "", false
	}
}

func toolArguments(item map[string]any) (map[string]any, string) {
	if arguments, ok := item["arguments"].(map[string]any); ok {
		return arguments, ""
	}
	raw, _ := item["arguments"].(string)
	arguments := map[string]any{}
	if json.Unmarshal([]byte(raw), &arguments) == nil {
		return arguments, raw
	}
	return map[string]any{}, raw
}

func firstRaw(values ...any) string {
	for _, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" && utf8.ValidString(text) {
			return text
		}
	}
	return ""
}

type pathAccess struct {
	path, workdir string
	wrote         bool
}

func collectFiles(paths []pathAccess, machineIdentity string) []File {
	if machineIdentity == "" {
		return nil
	}
	byDigest := map[string]File{}
	for _, candidate := range paths {
		path := strings.Trim(strings.TrimSpace(candidate.path), "\"'")
		if path == "" || path == "-" || path == "/dev/null" {
			continue
		}
		if !filepath.IsAbs(path) {
			if !filepath.IsAbs(candidate.workdir) {
				continue
			}
			path = filepath.Join(candidate.workdir, path)
		}
		path = filepath.Clean(path)
		base := truncateUTF8(filepath.Base(path), 128)
		if base == "" || base == "." || base == string(filepath.Separator) {
			continue
		}
		digest := sha256.Sum256([]byte(machineIdentity + "\x00" + path))
		key := hex.EncodeToString(digest[:])
		file := byDigest[key]
		file.PathDigest, file.BaseName = key, base
		file.Wrote = file.Wrote || candidate.wrote
		byDigest[key] = file
	}
	result := make([]File, 0, len(byDigest))
	for _, file := range byDigest {
		result = append(result, file)
	}
	sortFiles(result)
	return result
}

func truncateUTF8(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	value = value[:limit]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func sortFiles(files []File) {
	for i := 1; i < len(files); i++ {
		for j := i; j > 0 && files[j].PathDigest < files[j-1].PathDigest; j-- {
			files[j], files[j-1] = files[j-1], files[j]
		}
	}
}

func extractPatchPaths(patch string) []pathAccess {
	var result []pathAccess
	for _, line := range strings.Split(patch, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"*** Update File:", "*** Add File:", "*** Delete File:"} {
			if strings.HasPrefix(line, prefix) {
				result = append(result, pathAccess{path: strings.TrimSpace(strings.TrimPrefix(line, prefix)), wrote: true})
				break
			}
		}
	}
	return result
}

func classifyShellCommands(commands []string, workdir, machineIdentity string) (string, []File, string) {
	if len(commands) == 0 {
		return "bash", nil, ""
	}
	allRead := true
	hint := ""
	var paths []pathAccess
	for _, command := range commands {
		segments := splitShellSegments(command)
		if len(segments) == 0 {
			allRead = false
			continue
		}
		for _, segment := range segments {
			tokens := shellFields(segment)
			commandTokens := commandTokens(tokens)
			if len(commandTokens) == 0 {
				allRead = false
				continue
			}
			segmentPaths := shellPaths(segment, commandTokens, workdir)
			paths = append(paths, segmentPaths...)
			if hint == "" {
				hint = commandHint(commandTokens)
			}
			if !readOnlyCommand(commandTokens) {
				allRead = false
			}
		}
	}
	files := collectFiles(paths, machineIdentity)
	for _, file := range files {
		if file.Wrote {
			return "edit", files, hint
		}
	}
	if allRead {
		return "read", files, hint
	}
	return "bash", files, hint
}

func splitShellSegments(command string) []string {
	var result []string
	start := 0
	var quote byte
	escaped := false
	for i := 0; i < len(command); i++ {
		char := command[i]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' || char == '`' {
			quote = char
			continue
		}
		separator := char == ';' || char == '|' || char == '\n'
		width := 1
		if char == '&' && i+1 < len(command) && command[i+1] == '&' {
			separator, width = true, 2
		}
		if separator {
			if segment := strings.TrimSpace(command[start:i]); segment != "" {
				result = append(result, segment)
			}
			i += width - 1
			start = i + 1
		}
	}
	if segment := strings.TrimSpace(command[start:]); segment != "" {
		result = append(result, segment)
	}
	return result
}

func shellFields(segment string) []string {
	var fields []string
	var builder strings.Builder
	var quote byte
	escaped := false
	flush := func() {
		if builder.Len() > 0 {
			fields = append(fields, builder.String())
			builder.Reset()
		}
	}
	for i := 0; i < len(segment); i++ {
		char := segment[i]
		if escaped {
			builder.WriteByte(char)
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				builder.WriteByte(char)
			}
			continue
		}
		if char == '\'' || char == '"' || char == '`' {
			quote = char
			continue
		}
		if char == '#' && builder.Len() == 0 && (i == 0 || segment[i-1] == ' ' || segment[i-1] == '\t') {
			break
		}
		if char == ' ' || char == '\t' || char == '\n' {
			flush()
			continue
		}
		if char == '>' || char == '<' {
			flush()
			op := string(char)
			if i+1 < len(segment) && segment[i+1] == char {
				op += string(char)
				i++
			}
			fields = append(fields, op)
			continue
		}
		builder.WriteByte(char)
	}
	flush()
	return fields
}

func commandTokens(tokens []string) []string {
	for len(tokens) > 0 {
		head := filepath.Base(tokens[0])
		if head == "sudo" || head == "command" || head == "builtin" {
			tokens = tokens[1:]
			continue
		}
		if head == "npx" {
			tokens = tokens[1:]
			for len(tokens) > 0 && strings.HasPrefix(tokens[0], "-") {
				tokens = tokens[1:]
			}
			continue
		}
		if head == "env" {
			tokens = tokens[1:]
			for len(tokens) > 0 && (strings.HasPrefix(tokens[0], "-") || assignment(tokens[0])) {
				tokens = tokens[1:]
			}
			continue
		}
		if assignment(tokens[0]) {
			tokens = tokens[1:]
			continue
		}
		break
	}
	return tokens
}

func assignment(token string) bool {
	index := strings.IndexByte(token, '=')
	if index <= 0 {
		return false
	}
	for _, char := range token[:index] {
		if !(char == '_' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9') {
			return false
		}
	}
	return true
}

// commandHint reduces a parsed command to the only two subcategory facts
// Decision 3 takes from a command string: whether it names a test runner, and
// whether it is chore-shaped. The command itself is dropped in the same function
// that read it, per Decision 2; what leaves is one of three constants.
func commandHint(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	command := filepath.Base(tokens[0])
	switch command {
	case "pytest", "jest", "vitest", "mocha", "rspec", "phpunit", "gradlew", "ctest":
		return HintTesting
	case "git":
		return HintChore
	case "make", "cmake", "bazel", "mvn", "gradle":
		return HintChore
	case "npm", "pnpm", "yarn", "bun", "cargo", "go", "poetry", "pip", "pip3", "uv", "bundle", "composer":
		for _, token := range tokens[1:] {
			if strings.HasPrefix(token, "-") {
				continue
			}
			switch token {
			case "test", "t":
				return HintTesting
			case "install", "add", "update", "upgrade", "ci", "sync", "tidy", "vendor", "build", "mod":
				return HintChore
			}
			break
		}
		return ""
	}
	return ""
}

func readOnlyCommand(tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	command := filepath.Base(tokens[0])
	readOnly := map[string]bool{
		"awk": true, "bat": true, "cat": true, "cut": true, "diff": true,
		"dust": true, "eza": true, "fd": true, "find": true, "grep": true,
		"head": true, "jq": true, "ls": true, "pwd": true, "rg": true,
		"sed": true, "sort": true, "stat": true, "tail": true, "test": true,
		"tr": true, "uniq": true, "wc": true, "which": true, "yq": true,
	}
	if command == "sed" {
		for _, token := range tokens[1:] {
			if token == "-i" || strings.HasPrefix(token, "-i") {
				return false
			}
		}
	}
	if command != "git" {
		return readOnly[command]
	}
	for _, token := range tokens[1:] {
		if strings.HasPrefix(token, "-") {
			continue
		}
		return map[string]bool{"blame": true, "branch": true, "diff": true, "grep": true, "log": true, "rev-parse": true, "show": true, "status": true, "tag": true}[token]
	}
	return false
}

func shellPaths(segment string, tokens []string, workdir string) []pathAccess {
	paths := extractPatchPaths(segment)
	for i := range paths {
		paths[i].workdir = workdir
	}
	if len(tokens) == 0 {
		return paths
	}
	command := filepath.Base(tokens[0])
	for index, token := range tokens {
		if token == ">" || token == ">>" {
			if index+1 < len(tokens) && !strings.HasPrefix(tokens[index+1], "&") {
				paths = append(paths, pathAccess{path: tokens[index+1], workdir: workdir, wrote: true})
			}
		} else if strings.HasPrefix(token, ">") && len(token) > 1 && !strings.HasPrefix(token[1:], "&") {
			paths = append(paths, pathAccess{path: strings.TrimLeft(token, ">"), workdir: workdir, wrote: true})
		}
	}
	lastArgument := func() string {
		for index := len(tokens) - 1; index > 0; index-- {
			if !strings.HasPrefix(tokens[index], "-") && tokens[index] != ">" && tokens[index] != ">>" {
				return tokens[index]
			}
		}
		return ""
	}
	switch command {
	case "cp", "mv":
		paths = append(paths, pathAccess{path: lastArgument(), workdir: workdir, wrote: true})
	case "tee":
		for _, token := range tokens[1:] {
			if !strings.HasPrefix(token, "-") {
				paths = append(paths, pathAccess{path: token, workdir: workdir, wrote: true})
			}
		}
	case "sed":
		if !readOnlyCommand(tokens) {
			paths = append(paths, pathAccess{path: lastArgument(), workdir: workdir, wrote: true})
		}
	case "cat", "bat", "head", "tail":
		for _, token := range tokens[1:] {
			if !strings.HasPrefix(token, "-") && token != ">" && token != ">>" {
				paths = append(paths, pathAccess{path: token, workdir: workdir})
			}
		}
	}
	return paths
}

func execCommandLiterals(payload string) []string {
	var commands []string
	const marker = "tools.exec_command"
	for search := 0; search < len(payload); {
		index := strings.Index(payload[search:], marker)
		if index < 0 {
			break
		}
		start := search + index + len(marker)
		for start < len(payload) && (payload[start] == ' ' || payload[start] == '\t') {
			start++
		}
		if start >= len(payload) || payload[start] != '(' {
			search = start
			continue
		}
		body, end, ok := jsCallBody(payload, start)
		if !ok {
			break
		}
		if cmd, ok := jsObjectString(body, "cmd"); ok {
			commands = append(commands, cmd)
		}
		search = end + 1
	}
	return commands
}

func jsCallBody(payload string, open int) (string, int, bool) {
	depth := 0
	var quote byte
	escaped := false
	for index := open; index < len(payload); index++ {
		char := payload[index]
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if char == '\\' {
				escaped = true
			} else if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' || char == '`' {
			quote = char
			continue
		}
		switch char {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return payload[open+1 : index], index, true
			}
		}
	}
	return "", 0, false
}

func jsObjectString(value, key string) (string, bool) {
	for _, needle := range []string{key, `"` + key + `"`, `'` + key + `'`} {
		for search := 0; search < len(value); {
			index := strings.Index(value[search:], needle)
			if index < 0 {
				break
			}
			position := search + index + len(needle)
			for position < len(value) && (value[position] == ' ' || value[position] == '\t') {
				position++
			}
			if position >= len(value) || value[position] != ':' {
				search = position
				continue
			}
			position++
			for position < len(value) && (value[position] == ' ' || value[position] == '\t') {
				position++
			}
			if position >= len(value) || (value[position] != '\'' && value[position] != '"' && value[position] != '`') {
				return "", false
			}
			return parseJSLiteral(value[position:])
		}
	}
	return "", false
}

func parseJSLiteral(value string) (string, bool) {
	quote := value[0]
	var builder strings.Builder
	escaped := false
	for index := 1; index < len(value); index++ {
		char := value[index]
		if escaped {
			switch char {
			case 'n':
				builder.WriteByte('\n')
			case 'r':
				builder.WriteByte('\r')
			case 't':
				builder.WriteByte('\t')
			default:
				builder.WriteByte(char)
			}
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char == quote {
			return builder.String(), true
		}
		builder.WriteByte(char)
	}
	return "", false
}

func ClaudeUserTurnBoundary(value, message map[string]any) bool {
	if safeString(value["type"]) != "user" && safeString(message["role"]) != "user" {
		return false
	}
	if meta, _ := value["isMeta"].(bool); meta {
		return false
	}
	items := contentItems(message["content"])
	for _, item := range items {
		if safeString(item["type"]) == "tool_result" {
			return false
		}
	}
	text := claudeMessageText(message["content"])
	if text == "" {
		return len(items) == 0
	}
	trimmed := strings.TrimSpace(text)
	for _, prefix := range []string{"<local-command-", "<system-reminder>", "<command-name>", "<command-message>", "<command-args>", "Base directory for this skill:", "[Image #"} {
		if strings.HasPrefix(trimmed, prefix) {
			return false
		}
	}
	return true
}

// ClaudeTurnMessage returns the text of a Claude entry that opens a turn, and
// whether it opens one at all. Callers outside this package get the text only
// long enough to reduce it: Decision 2 permits reading a user message and
// forbids keeping it.
func ClaudeTurnMessage(value, message map[string]any) (string, bool) {
	if !ClaudeUserTurnBoundary(value, message) {
		return "", false
	}
	return claudeMessageText(message["content"]), true
}

func claudeMessageText(content any) string {
	if text, ok := content.(string); ok {
		return text
	}
	var parts []string
	for _, item := range contentItems(content) {
		if safeString(item["type"]) == "text" {
			if text, ok := item["text"].(string); ok {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}
