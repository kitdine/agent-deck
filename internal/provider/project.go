package provider

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/kitdine/agent-deck/internal/session"
)

const (
	HeadroomProjectEnvironment     = "HEADROOM_PROJECT"
	HeadroomProjectHeader          = "X-Headroom-Project"
	ClaudeCustomHeadersEnvironment = "ANTHROPIC_CUSTOM_HEADERS"
	codexProjectHeadersMappingTOML = `env_http_headers = { "X-Headroom-Project" = "HEADROOM_PROJECT" }`
)

// ProjectIdentity returns the same cleaned directory identity stored for a
// client session.
func ProjectIdentity(cwd string) string {
	return session.NormalizeProject(cwd)
}

// ProjectWireValue returns the percent-encoded base name used for wrapper
// attribution. Filesystem roots and the "." and ".." directory references have
// no name of their own and therefore produce no attribution value.
func ProjectWireValue(cwd string) string {
	identity := ProjectIdentity(cwd)
	if identity == "" {
		return ""
	}
	name := filepath.Base(identity)
	if name == "." || name == ".." || name == string(filepath.Separator) {
		return ""
	}
	escaped := url.PathEscape(name)
	// PathEscape leaves "+" unescaped, while form-style decoders interpret it
	// as a space. Encoding it explicitly makes both decoder families agree.
	return strings.ReplaceAll(escaped, "+", "%2B")
}

func injectProjectEnvironment(client Client, environ []string, value string) ([]string, bool) {
	if value == "" {
		return nil, false
	}
	switch client {
	case ClientCodex:
		if _, found := environmentValue(environ, HeadroomProjectEnvironment); found {
			return nil, false
		}
		return appendEnvironment(environ, HeadroomProjectEnvironment, value), true
	case ClientClaude:
		current, found := environmentValue(environ, ClaudeCustomHeadersEnvironment)
		if found && containsHTTPHeader(current, HeadroomProjectHeader) {
			return nil, false
		}
		header := HeadroomProjectHeader + ": " + value
		if found && current != "" && !strings.HasSuffix(current, "\n") {
			header = current + "\n" + header
		} else if found {
			header = current + header
		}
		return replaceEnvironment(environ, ClaudeCustomHeadersEnvironment, header), true
	default:
		return nil, false
	}
}

func environmentValue(environ []string, name string) (string, bool) {
	prefix := name + "="
	for i := len(environ) - 1; i >= 0; i-- {
		if strings.HasPrefix(environ[i], prefix) {
			return strings.TrimPrefix(environ[i], prefix), true
		}
	}
	return "", false
}

func appendEnvironment(environ []string, name, value string) []string {
	result := append([]string(nil), environ...)
	return append(result, name+"="+value)
}

func replaceEnvironment(environ []string, name, value string) []string {
	prefix := name + "="
	result := append([]string(nil), environ...)
	for i := len(result) - 1; i >= 0; i-- {
		if strings.HasPrefix(result[i], prefix) {
			result[i] = prefix + value
			return result
		}
	}
	return append(result, prefix+value)
}

func containsHTTPHeader(value, name string) bool {
	for _, line := range strings.Split(value, "\n") {
		field, _, found := strings.Cut(line, ":")
		if found && strings.EqualFold(strings.TrimSpace(field), name) {
			return true
		}
	}
	return false
}
