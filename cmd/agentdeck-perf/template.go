package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// templateFiles are the state root files a template carries. Databases are
// copied with VACUUM INTO (a consistent snapshot without WAL); caches by bytes.
var (
	templateDatabases = []string{"agentdeck.sqlite3", "sessions.sqlite3"}
	templateCaches    = []string{"desktop-serving-snapshot.json", "desktop-derived-cache.json", "scan-receipts.json"}
)

type templateManifest struct {
	CoreSHA256    string `json:"core_sha256"`
	SessionSHA256 string `json:"session_sha256"`
	ServingSHA256 string `json:"serving_sha256,omitempty"`
}

func runTemplate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("template", flag.ContinueOnError)
	flags.SetOutput(stderr)
	agentdeck := flags.String("agentdeck", "", "agentdeck binary under test")
	home := flags.String("home", "", "fixture HOME")
	tz := flags.String("tz", "America/New_York", "timezone the product runs in")
	out := flags.String("out", "", "empty directory that receives the template")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	manifest, err := buildTemplate(*agentdeck, *home, *tz, *out)
	if err == nil {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(manifest)
	}
	if err != nil {
		fmt.Fprintln(stderr, "agentdeck-perf template:", err)
		return 1
	}
	return 0
}

// buildTemplate lets the product itself import the fixture into an isolated
// state root, then copies that state as the read-only batch template.
func buildTemplate(agentdeck, home, tz, out string) (templateManifest, error) {
	for _, path := range []string{agentdeck, home, out} {
		if !filepath.IsAbs(path) {
			return templateManifest{}, errors.New("--agentdeck, --home and --out must be absolute")
		}
	}
	if err := requireEmptyDir(out); err != nil {
		return templateManifest{}, err
	}
	state, err := os.MkdirTemp("", "agentdeck-perf-template-")
	if err != nil {
		return templateManifest{}, err
	}
	defer os.RemoveAll(state)
	command := exec.Command(agentdeck, "--state-dir", state, "--format", "json", "scan")
	command.Env = []string{"HOME=" + home, "TZ=" + tz, "PATH=/usr/bin:/bin", "LANG=en_US_POSIX", "LC_ALL=en_US_POSIX"}
	addTestCLIEnvironment(command, command.Path)
	if output, err := command.CombinedOutput(); err != nil {
		return templateManifest{}, fmt.Errorf("product scan failed: %v (%d bytes of output)", err, len(output))
	}
	var manifest templateManifest
	for _, name := range templateDatabases {
		target := filepath.Join(out, name)
		if err := vacuumInto(filepath.Join(state, name), target); err != nil {
			return templateManifest{}, err
		}
		sum, err := fileSHA256(target)
		if err != nil {
			return templateManifest{}, err
		}
		if name == "agentdeck.sqlite3" {
			manifest.CoreSHA256 = sum
		} else {
			manifest.SessionSHA256 = sum
		}
	}
	for _, name := range templateCaches {
		raw, err := os.ReadFile(filepath.Join(state, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return templateManifest{}, err
		}
		if err := os.WriteFile(filepath.Join(out, name), raw, 0o600); err != nil {
			return templateManifest{}, err
		}
		if name == "desktop-serving-snapshot.json" {
			sum := sha256.Sum256(raw)
			manifest.ServingSHA256 = hex.EncodeToString(sum[:])
		}
	}
	return manifest, nil
}

func vacuumInto(source, target string) error {
	db, err := sql.Open("sqlite", "file:"+source+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec("VACUUM INTO ?", target)
	if err == nil {
		err = os.Chmod(target, 0o600)
	}
	return err
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
