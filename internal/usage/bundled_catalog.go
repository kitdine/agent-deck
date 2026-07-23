package usage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// BundledCatalogVersionDigestLength is how many hex characters of the bundled
// catalog's content digest are carried in its catalog_version string.
const BundledCatalogVersionDigestLength = 12

// bundledCatalogVersionSeparator precedes the content digest inside a bundled
// catalog_version string, e.g. "2026-07-23-litellm-merged-g1a2b3c4d5e6f".
const bundledCatalogVersionSeparator = "-g"

// BundledCatalogVersionDigest returns the content digest that a bundled price
// catalog's catalog_version must carry.
//
// importCatalog inserts with INSERT OR IGNORE keyed on the catalog version, so
// an existing install silently keeps stale prices — and a stale content_sha256
// — whenever the bundled file's contents change but its version string does
// not. Deriving the version from the content makes that failure impossible to
// ship: change any price and the required version changes with it.
//
// The digest is taken over the catalog's canonical JSON with catalog_version
// itself blanked (the field cannot contribute to a digest it carries) and with
// object keys sorted, so it tracks semantic content — models, providers,
// prices, aliases, effective dates — rather than byte-level formatting. The
// raw-file content_sha256 recorded at import time is a separate value and is
// unaffected by this normalization.
func BundledCatalogVersionDigest(data []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		return "", err
	}
	if _, ok := document["catalog_version"]; !ok {
		return "", fmt.Errorf("price catalog has no catalog_version")
	}
	document["catalog_version"] = ""
	// json.Marshal sorts map keys, so this canonical form is deterministic.
	canonical, err := json.Marshal(document)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])[:BundledCatalogVersionDigestLength], nil
}

// BundledCatalogVersionFor returns the catalog_version string that data must
// carry: its existing human-readable prefix with the current content digest
// appended. Regenerating the bundled catalog uses this to stamp the version.
func BundledCatalogVersionFor(data []byte, prefix string) (string, error) {
	digest, err := BundledCatalogVersionDigest(data)
	if err != nil {
		return "", err
	}
	return prefix + bundledCatalogVersionSeparator + digest, nil
}

// VerifyBundledCatalogVersion reports whether data's catalog_version carries
// the content digest its own contents require. It is the guard that keeps the
// "a bundled content change always carries a new catalog version string"
// invariant enforceable at test and release time rather than discovered as
// stale prices on an upgraded install.
func VerifyBundledCatalogVersion(data []byte) error {
	parsed, err := parseCatalog(data)
	if err != nil {
		return err
	}
	digest, err := BundledCatalogVersionDigest(data)
	if err != nil {
		return err
	}
	prefix, carried, found := strings.Cut(parsed.Version, bundledCatalogVersionSeparator)
	if !found {
		return fmt.Errorf("bundled catalog version %q carries no %q content digest; expected %q", parsed.Version, bundledCatalogVersionSeparator, parsed.Version+bundledCatalogVersionSeparator+digest)
	}
	if carried != digest {
		return fmt.Errorf("bundled catalog version %q carries stale content digest %q, want %q (contents changed without a version bump; expected version %q)", parsed.Version, carried, digest, prefix+bundledCatalogVersionSeparator+digest)
	}
	return nil
}

// VerifyBundledCatalog checks the catalog compiled into this binary.
func VerifyBundledCatalog() error { return VerifyBundledCatalogVersion(bundledCatalog) }
