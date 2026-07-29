package provider

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/kitdine/agent-deck/internal/session"
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
