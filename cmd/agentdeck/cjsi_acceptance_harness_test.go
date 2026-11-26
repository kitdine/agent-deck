package main

// Harness for TestCJSISourceIssueAcceptance (Task 10 source-issue-acceptance).
//
// Every CLI subprocess, the old baseline build included, runs with an empty
// environment except PATH, an isolated HOME, XDG config/data/cache, TMPDIR
// (which also places the scan-worker socket), a fixed TZ and an explicit
// --state-dir inside one fixture root. Nothing is inherited from the real
// Codex or Claude configuration or from real AgentDeck state. Fault paths use
// handshakes: a subprocess reports that it reached a boundary on stdout and the
// test then kills that process only. No sleep or elapsed-time threshold decides
// an ordering; the context deadlines below only stop a hung subprocess.

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	// cjsiAccOldBinaryEnv and cjsiAccCandidateBinaryEnv name externally built
	// binaries. tasks.md's combined command builds the old one from
	// 4ac56db32e13c4e925b648c620ba76fc588a7781 with `git archive`; see
	// testdata/cjsi_acceptance/build-binaries.sh.
	cjsiAccOldBinaryEnv       = "CJSI_OLD_BINARY"
	cjsiAccCandidateBinaryEnv = "CJSI_CANDIDATE_BINARY"
	cjsiAccOldBaselineCommit  = "4ac56db32e13c4e925b648c620ba76fc588a7781"

	// Hang guards only. A subprocess that is still running at the deadline is
	// a failure, never an accepted ordering.
	cjsiAccCommandGuard = 2 * time.Minute
	cjsiAccBuildGuard   = 10 * time.Minute

	cjsiAccPassphrase = "synthetic-acceptance-passphrase"
)

// cjsiAccRoot is one isolated fixture. All paths except tmp are inside dir;
// tmp is a short private directory the harness allocates (see
// cjsiAccShortTemp) so the scan-worker socket never needs a fallback.
type cjsiAccRoot struct {
	dir, home, config, data, cache, tmp, state string
}

func newCJSIAccRoot(t *testing.T, name string) *cjsiAccRoot {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, name)
	r := &cjsiAccRoot{
		dir:    dir,
		home:   filepath.Join(dir, "home"),
		config: filepath.Join(dir, "xdg-config"),
		data:   filepath.Join(dir, "xdg-data"),
		cache:  filepath.Join(dir, "xdg-cache"),
		tmp:    cjsiAccShortTemp(t),
		state:  filepath.Join(dir, "state"),
	}
	for _, path := range []string{r.home, r.config, r.data, r.cache} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

// cjsiAccSocketLimit is sockaddr_un's path limit on Darwin. The existing scan
// client (old and new builds) puts its socket at $TMPDIR/ad-s/<24-char id>/w
// and falls back to a compact /tmp/ad-s-<uid>-<hex> directory, recorded in
// <state>/.scan-runtime-parent, only when that path would reach the limit.
const cjsiAccSocketLimit = 104

var cjsiAccUserTemp = sync.OnceValues(func() (string, error) {
	out, err := exec.Command("/usr/bin/getconf", "DARWIN_USER_TEMP_DIR").Output()
	if err != nil {
		return "", err
	}
	// Keep the spelling getconf returns (/var/folders/...): the socket path
	// is built from $TMPDIR as given, and the resolved /private/var form is
	// 8 bytes longer. The resolved form is only used to refuse /tmp.
	dir := filepath.Clean(strings.TrimSpace(string(out)))
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(dir) || resolved == "/private/tmp" || strings.HasPrefix(resolved, "/private/tmp/") {
		return "", fmt.Errorf("unexpected per-user temporary directory %q (%q)", dir, resolved)
	}
	return dir, nil
})

// cjsiAccShortTemp allocates a fresh 0700 directory with os.MkdirTemp inside
// the account's private per-user temporary directory (DARWIN_USER_TEMP_DIR,
// not /tmp). It is short enough that the scan socket path stays below
// sockaddr_un's limit, so no fixture subprocess ever falls back to /tmp. The
// cleanup removes only this directory, and only while it still has the
// device and inode recorded when the harness created it.
func cjsiAccShortTemp(t *testing.T) string {
	t.Helper()
	parent, err := cjsiAccUserTemp()
	if err != nil {
		t.Fatalf("per-user temporary directory: %v", err)
	}
	dir, err := os.MkdirTemp(parent, "cjsi-")
	if err != nil {
		t.Fatal(err)
	}
	if socket := filepath.Join(dir, "ad-s", strings.Repeat("f", 24), "w"); len(socket) >= cjsiAccSocketLimit {
		_ = os.Remove(dir)
		t.Fatalf("socket path %d bytes would force the /tmp fallback: %s", len(socket), socket)
	}
	created, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		now, err := os.Lstat(dir)
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		if err != nil || !now.IsDir() || !os.SameFile(created, now) {
			t.Errorf("short temp %s is no longer the directory the harness created: %v", dir, err)
			return
		}
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("remove short temp: %v", err)
		}
	})
	return dir
}

// env is the complete subprocess environment; os.Environ is never inherited.
func (r *cjsiAccRoot) env() []string {
	return []string{
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
		"HOME=" + r.home,
		"XDG_CONFIG_HOME=" + r.config,
		"XDG_DATA_HOME=" + r.data,
		"XDG_CACHE_HOME=" + r.cache,
		"TMPDIR=" + r.tmp + "/",
		"TZ=UTC",
		"LANG=C",
		"NO_COLOR=1",
		"CFFIXED_USER_HOME=" + r.home,
	}
}

func (r *cjsiAccRoot) write(t *testing.T, rel, content string) string {
	t.Helper()
	path := filepath.Join(r.home, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type cjsiAccResult struct {
	args           []string
	exit           int
	stdout, stderr string
}

func (r cjsiAccResult) String() string {
	return fmt.Sprintf("%q exit=%d stdout=%s stderr=%s", r.args, r.exit, cjsiAccClip(r.stdout), cjsiAccClip(r.stderr))
}

func cjsiAccClip(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 600 {
		return s[:600] + "..."
	}
	return s
}

// errorCode returns the JSON error envelope code, from stderr or stdout.
func (r cjsiAccResult) errorCode() string {
	for _, stream := range []string{r.stderr, r.stdout} {
		for _, line := range strings.Split(stream, "\n") {
			var envelope struct {
				Error *struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal([]byte(line), &envelope) == nil && envelope.Error != nil {
				return envelope.Error.Code
			}
		}
	}
	return ""
}

func (r cjsiAccResult) errorMessage() string {
	for _, stream := range []string{r.stderr, r.stdout} {
		for _, line := range strings.Split(stream, "\n") {
			var envelope struct {
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal([]byte(line), &envelope) == nil && envelope.Error != nil {
				return envelope.Error.Message
			}
		}
	}
	return ""
}

func (r cjsiAccResult) data(t *testing.T) json.RawMessage {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &envelope); err != nil {
		t.Fatalf("decode %s: %v", r, err)
	}
	return envelope.Data
}

// run executes binary against this root with --state-dir and JSON output,
// then waits for any detached scan worker that the command started for this
// root to release the scan lock, so later snapshots see a quiescent root.
func (r *cjsiAccRoot) run(t *testing.T, binary, stdin string, args ...string) cjsiAccResult {
	t.Helper()
	return r.runFormat(t, binary, "json", stdin, args...)
}

func (r *cjsiAccRoot) runFormat(t *testing.T, binary, format, stdin string, args ...string) cjsiAccResult {
	t.Helper()
	full := append([]string{"--state-dir", r.state, "--format", format}, args...)
	ctx, cancel := context.WithTimeout(context.Background(), cjsiAccCommandGuard)
	defer cancel()
	cmd := cjsiAccGuarded(ctx, t, binary, full...)
	cmd.Env = r.env()
	cmd.Dir = r.dir
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := cjsiAccResult{args: args, stdout: stdout.String(), stderr: stderr.String()}
	cjsiAccAssertNoHostProbe(t, result.stdout+result.stderr)
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr) && ctx.Err() == nil:
		result.exit = exitErr.ExitCode()
	default:
		t.Fatalf("%s %q did not finish: %v (ctx %v)", filepath.Base(binary), args, err, ctx.Err())
	}
	r.quiesce(t)
	return result
}

// quiesce waits, without taking or reclaiming any lock, until no fixture
// process can still write this root. It observes state.lock and scan.lock:
// a well-formed token whose owner process is alive is a running worker (or
// the command's own detached round), so the harness waits for that process
// to release the lock and then to exit. A lock whose owner is gone is left in
// place untouched for the snapshot to judge, so a lock an old process leaves
// behind is reported as a write instead of being silently reclaimed. The wait
// is bounded by cjsiAccCommandGuard and ends on an observed state change.
func (r *cjsiAccRoot) quiesce(t *testing.T) {
	t.Helper()
	if err := r.quiesceErr(); err != nil {
		t.Fatal(err)
	}
}

// quiesceErr is quiesce without the test handle. A lock path that is not a
// regular file (FIFO, directory, symlink, device, socket) is reported at
// once: the protocol only ever writes regular files, and the harness never
// opens such an entry, so a swapped-in FIFO cannot block the observer.
func (r *cjsiAccRoot) quiesceErr() error {
	deadline := time.Now().Add(cjsiAccCommandGuard)
	owners := map[int]bool{}
	for {
		busy := false
		for _, name := range []string{"state.lock", "scan.lock"} {
			content, _, err := cjsiAccReadRegular(filepath.Join(r.state, name), cjsiAccLockReadLimit)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return fmt.Errorf("fixture lock %s violates the lock protocol: %w", name, err)
			}
			if pid, ok := cjsiAccLockOwner(content); ok && cjsiAccAlive(pid) {
				owners[pid] = true
				busy = true
			}
		}
		for pid := range owners {
			if cjsiAccAlive(pid) {
				busy = true
			}
		}
		if !busy {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("fixture lock owners %v still running after %v", owners, cjsiAccCommandGuard)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

const (
	// The lock protocol writes at most 64 bytes; anything longer is not a
	// token, and the observer never needs more than this to decide.
	cjsiAccLockReadLimit = 4096
	// Upper bound for any fixture state file the harness reads.
	cjsiAccStateReadLimit = 256 << 20
)

// errCJSIAccNotRegular reports a fixture path that is not a regular file.
var errCJSIAccNotRegular = errors.New("not a regular file")

// cjsiAccReadRegular reads a fixture state file without ever blocking on or
// following something that is not a regular file: it lstat's the path and
// refuses any non-regular entry, opens it O_RDONLY|O_NONBLOCK|O_NOFOLLOW,
// checks with fstat that the open descriptor is still that same regular file
// (device and inode), and reads at most limit bytes (more is an error).
func cjsiAccReadRegular(path string, limit int64) ([]byte, os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, info, fmt.Errorf("%s is %v: %w", path, info.Mode().Type(), errCJSIAccNotRegular)
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, info, &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, info, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, info, fmt.Errorf("%s changed identity while it was opened: %w", path, errCJSIAccNotRegular)
	}
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, info, err
	}
	if int64(len(content)) > limit {
		return nil, info, fmt.Errorf("%s exceeds %d bytes", path, limit)
	}
	return content, opened, nil
}

func cjsiAccAlive(pid int) bool {
	return !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
}

// cjsiAccLockOwner parses a lock token "v1:<pid>:<32 hex>".
func cjsiAccLockOwner(content []byte) (int, bool) {
	match := cjsiAccLockToken.FindSubmatch(content)
	if match == nil {
		return 0, false
	}
	pid, err := strconv.Atoi(string(match[1]))
	return pid, err == nil
}

// cjsiAccKnownStaleLocks holds the identity (path, inode, token) of every
// lock the harness itself left stale by killing the upgrader at its pause.
// Only these may disappear between snapshots: the store's lock protocol of
// the next acquirer reclaims them. Any other lock that appears, changes or
// disappears is reported.
var cjsiAccKnownStaleLocks sync.Map

func cjsiAccLockKey(path string, ino uint64, content []byte) string {
	return fmt.Sprintf("%s\x00%d\x00%s", path, ino, content)
}

// recordUpgraderLocks records the locks the killed upgrader (pid) left.
func (r *cjsiAccRoot) recordUpgraderLocks(t *testing.T, pid int) {
	t.Helper()
	for _, name := range []string{"state.lock", "scan.lock"} {
		path := filepath.Join(r.state, name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		content, _, err := cjsiAccReadRegular(path, cjsiAccLockReadLimit)
		if err != nil {
			t.Fatal(err)
		}
		owner, ok := cjsiAccLockOwner(content)
		st, isStat := info.Sys().(*syscall.Stat_t)
		if !ok || owner != pid || cjsiAccAlive(owner) || !isStat || !info.Mode().IsRegular() {
			t.Fatalf("%s after killing upgrader %d is not its stale lock: %q", name, pid, content)
		}
		cjsiAccKnownStaleLocks.Store(cjsiAccLockKey(path, st.Ino, content), true)
		t.Logf("recorded the killed upgrader's stale %s", name)
	}
}

// Host isolation guard. The old build's doctor (and desktop snapshot health)
// runs a LaunchServices/PlugInKit registration probe through the absolute
// paths /usr/bin/osascript and /usr/bin/pluginkit, which read the host
// /Applications/AgentDeck.app and PlugInKit registrations; a PATH sentinel
// cannot intercept absolute paths. Every fixture subprocess, including the
// scan workers and probes it spawns, therefore runs under sandbox-exec with
// a profile that refuses to execute those tools and refuses any access to
// the host app, the real App Group container and the real ~/.agentdeck,
// ~/.codex and ~/.claude. Any attempt fails inside the sandbox instead of
// touching the host, and cjsiAccAssertNoHostProbe fails the test when the
// probe's "launchservices" check shows up in an answer at all.
const cjsiAccSandboxExec = "/usr/bin/sandbox-exec"

var cjsiAccHostTools = []string{
	"/usr/bin/osascript",
	"/usr/bin/pluginkit",
	"/usr/bin/open",
	"/System/Library/Frameworks/CoreServices.framework/Versions/A/Frameworks/LaunchServices.framework/Versions/A/Support/lsregister",
}

func cjsiAccSandboxQuote(path string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(path) + `"`
}

// cjsiAccGuardProfile is the sandbox profile; it uses the account's real home
// from the user database, not $HOME, which tests may have redirected.
//
//   - The registration tools may not be executed at all.
//   - The host app may not be read or written at all, metadata included.
//   - The real App Group, ~/.agentdeck, ~/.codex and ~/.claude may not have
//     any content read or written, and nothing below them may even be
//     stat'ed. Only the entrance's own metadata stays readable: the state
//     ownership guard of both builds lstat's the real ~/.agentdeck entrance
//     to refuse a --state-dir that aliases it ("never descendant metadata"
//     in internal/platform), and fails closed without it.
//   - The real ~/.claude.json (Claude user-level MCP configuration) and its
//     backup, the shell start-up files and ~/.config/fish are closed
//     completely (cjsiAccGuardClosedFiles, cjsiAccGuardClosedDirs).
//   - The real installation directory ~/Library/Application Support/AgentDeck
//     (installation.json receipt, ownership.lock) is closed completely,
//     metadata included. With an explicit isolated --state-dir neither build
//     needs it: the ownership guard returns before resolving an installation,
//     and the old doctor's installation check then reports a refused read
//     instead of reading the receipt or lstat'ing the executable it names.
//
// The profile starts from (allow default): it is a targeted guard against
// these known host contacts, not general isolation. Anything not listed
// (other host files, network) is not prevented by it.
func cjsiAccGuardProfile(t *testing.T) string {
	t.Helper()
	account, err := user.Current()
	if err != nil || account.HomeDir == "" {
		t.Fatalf("resolve the real home for the host guard: %v", err)
	}
	return cjsiAccGuardProfileFor(account.HomeDir)
}

// cjsiAccGuardPrivateDirs are the account-private locations the guard closes.
func cjsiAccGuardPrivateDirs(home string) []string {
	return []string{
		filepath.Join(home, "Library", "Group Containers", "N2FZ2FNRTU.group.com.kitdine.agentdeck"),
		filepath.Join(home, ".agentdeck"),
		filepath.Join(home, ".codex"),
		filepath.Join(home, ".claude"),
	}
}

// cjsiAccGuardClosedDirs are closed completely, metadata included.
func cjsiAccGuardClosedDirs(home string) []string {
	return []string{
		"/Applications/AgentDeck.app",
		filepath.Join(home, "Library", "Application Support", "AgentDeck"),
		// Shell integration's fish configuration directory.
		filepath.Join(home, ".config", "fish"),
	}
}

// cjsiAccGuardClosedFiles are single files in the real home that AgentDeck
// reads or edits relative to the home it resolves, and that no directory
// rule covers: the Claude user-level configuration (MCP servers) and its
// backup, and the shell start-up files shell integration manages. Codex's
// config.toml/hooks.json and Claude's settings.json, plugins and skills are
// already covered by the ~/.codex and ~/.claude rules.
func cjsiAccGuardClosedFiles(home string) []string {
	var files []string
	for _, name := range []string{".claude.json", ".claude.json.backup", ".bashrc", ".bash_profile", ".zshrc", ".zprofile", ".zshenv", ".profile"} {
		files = append(files, filepath.Join(home, name))
	}
	return files
}

func cjsiAccGuardProfileFor(home string) string {
	var tools []string
	for _, tool := range cjsiAccHostTools {
		tools = append(tools, "(literal "+cjsiAccSandboxQuote(tool)+")")
	}
	var private, below []string
	for _, path := range cjsiAccGuardPrivateDirs(home) {
		private = append(private, "(subpath "+cjsiAccSandboxQuote(path)+")")
		below = append(below, "(regex "+cjsiAccSandboxQuote("^"+regexp.QuoteMeta(path)+"/")+")")
	}
	var closed []string
	for _, path := range cjsiAccGuardClosedDirs(home) {
		closed = append(closed, "(subpath "+cjsiAccSandboxQuote(path)+")")
	}
	for _, path := range cjsiAccGuardClosedFiles(home) {
		closed = append(closed, "(literal "+cjsiAccSandboxQuote(path)+")")
	}
	return "(version 1)\n(allow default)\n" +
		"(deny process-exec* " + strings.Join(tools, " ") + ")\n" +
		"(deny file-read* file-write* " + strings.Join(closed, " ") + ")\n" +
		"(deny file-read-data file-read-xattr file-write* " + strings.Join(private, " ") + ")\n" +
		"(deny file-read-metadata " + strings.Join(below, " ") + ")\n"
}

// cjsiAccGuarded builds a fixture subprocess under the host guard.
func cjsiAccGuarded(ctx context.Context, t *testing.T, binary string, args ...string) *exec.Cmd {
	t.Helper()
	return cjsiAccGuardedWith(ctx, t, cjsiAccGuardProfile(t), binary, args...)
}

func cjsiAccGuardedWith(ctx context.Context, t *testing.T, profile, binary string, args ...string) *exec.Cmd {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("the host guard and the fixture subprocesses are macOS-only")
	}
	if _, err := os.Stat(cjsiAccSandboxExec); err != nil {
		t.Fatalf("host guard unavailable: %v", err)
	}
	return exec.CommandContext(ctx, cjsiAccSandboxExec, append([]string{"-p", profile, binary}, args...)...)
}

// cjsiAccAssertNoHostProbe fails when an answer carries the registration
// probe's check: the probe ran (and, under the guard, could only fail).
func cjsiAccAssertNoHostProbe(t *testing.T, output string) {
	t.Helper()
	at := strings.Index(output, `"launchservices"`)
	if at < 0 {
		at = strings.Index(output, "launchservices_registration")
	}
	if at >= 0 {
		t.Fatalf("a fixture subprocess reached the host LaunchServices/PlugInKit registration probe: ...%s", cjsiAccClip(output[max(0, at-40):]))
	}
}

// cjsiAccFile is one entry of a state root: identity and exact content of
// every regular file. Only the core database additionally carries a logical
// digest (see cjsiAccCoreRule).
type cjsiAccFile struct {
	size    int64
	ino     uint64
	mode    os.FileMode
	sha     string
	logical string
	// content is kept for the protocol files whose change is validated by
	// meaning (lock tokens, the scan receipt journal); home is the fixture
	// home a receipt must name.
	content []byte
	home    string
	path    string
}

type cjsiAccSnapshot map[string]cjsiAccFile

const cjsiAccCoreName = "agentdeck.sqlite3"

func cjsiAccSnapshotState(t *testing.T, root string) cjsiAccSnapshot {
	t.Helper()
	snap := cjsiAccSnapshot{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		file := cjsiAccFile{size: info.Size(), mode: info.Mode()}
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			file.ino = st.Ino
		}
		if d.IsDir() {
			snap[rel+"/"] = file
			return nil
		}
		if info.Mode().IsRegular() {
			content, opened, err := cjsiAccReadRegular(path, cjsiAccStateReadLimit)
			if err != nil {
				return err
			}
			if st, ok := opened.Sys().(*syscall.Stat_t); ok {
				file.ino, file.size, file.mode = st.Ino, opened.Size(), opened.Mode()
			}
			sum := sha256.Sum256(content)
			file.sha = hex.EncodeToString(sum[:])
			switch cjsiAccRuleFor(rel) {
			case cjsiAccCoreRule:
				file.logical = cjsiAccLogicalDigest(t, path)
			case cjsiAccLock, cjsiAccReceipts:
				file.content = content
				file.home = filepath.Join(filepath.Dir(root), "home")
				file.path = path
			}
		}
		snap[rel] = file
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

// cjsiAccLogicalDigest copies the core database with its -wal/-journal to a
// private directory and hashes journal_mode, application_id, user_version,
// sqlite_master, and every row of every ordinary table including the implicit
// rowid of rowid tables (FTS shadow tables included; virtual tables are
// covered by their shadows). The fixture root itself is never opened.
func cjsiAccLogicalDigest(t *testing.T, path string) string {
	t.Helper()
	dir := t.TempDir()
	copyPath := filepath.Join(dir, "db.sqlite3")
	for _, suffix := range []string{"", "-wal", "-journal"} {
		content, _, err := cjsiAccReadRegular(path+suffix, cjsiAccStateReadLimit)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) && suffix != "" {
				continue
			}
			t.Fatalf("core database copy: %v", err)
		}
		if err := os.WriteFile(copyPath+suffix, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: copyPath}).String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	digest := sha256.New()
	query := func(q string) [][]string {
		rows, err := db.Query(q)
		if err != nil {
			t.Fatalf("%s: %s: %v", path, q, err)
		}
		defer rows.Close()
		columns, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		var out [][]string
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err := rows.Scan(pointers...); err != nil {
				t.Fatal(err)
			}
			row := make([]string, len(values))
			for i, v := range values {
				row[i] = fmt.Sprintf("%T:%v", v, v)
			}
			out = append(out, row)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	tables := query(`SELECT type, name, tbl_name, coalesce(sql, '') FROM sqlite_master ORDER BY type, name`)
	fmt.Fprintf(digest, "%v\n%v\n%v\n%v\n", query(`PRAGMA journal_mode`), query(`PRAGMA application_id`), query(`PRAGMA user_version`), tables)
	for _, table := range tables {
		ddl := strings.ToUpper(table[3])
		if table[0] != "string:table" || strings.Contains(ddl, "CREATE VIRTUAL TABLE") {
			continue
		}
		name := `"` + strings.ReplaceAll(strings.TrimPrefix(table[1], "string:"), `"`, `""`) + `"`
		selectList := "rowid, *"
		if strings.Contains(ddl, "WITHOUT ROWID") {
			selectList = "*"
		}
		rows := query(`SELECT ` + selectList + ` FROM ` + name)
		encoded := make([]string, len(rows))
		for i, row := range rows {
			encoded[i] = strings.Join(row, "\x1f")
		}
		sort.Strings(encoded)
		fmt.Fprintf(digest, "%s\n%s\n", name, strings.Join(encoded, "\x1e"))
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func (s cjsiAccSnapshot) names() []string {
	names := make([]string, 0, len(s))
	for name := range s {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// cjsiAccRule says how one state-root path may differ between two snapshots.
// Paths not listed are compared exactly (inode, size, mode, SHA-256), must not
// disappear, and must not newly appear. That covers the private index and its
// (forbidden) sidecars, the fence, the legacy snapshot, the legacy-drain
// directory, the scan-runtime locator (never created: the harness keeps the
// socket path short), receipt-journal temporaries (renamed or removed before
// a command returns) and any other file or directory.
type cjsiAccRule int

const (
	cjsiAccExact cjsiAccRule = iota
	// The core database: same inode and mode, equal logical digest. Its byte
	// image may change because SQLite checkpoints pending WAL frames when
	// any opener (old or new) closes its last connection.
	cjsiAccCoreRule
	// Folded into the core logical digest: the core WAL may grow, shrink,
	// appear or disappear with a checkpoint.
	cjsiAccCoreWAL
	// SQLite's wal-index for the core: rebuilt by any WAL opener and never
	// a source of data; it must stay a 0600 regular file.
	cjsiAccCoreSHM
	// state.lock / scan.lock: the store's lock protocol creates the entry
	// with O_EXCL holding "v1:<pid>:<32 hex>", unlinks it on release, and
	// reclaims an entry whose owner process is dead. quiesce never takes or
	// reclaims a lock. At a quiescent snapshot a lock is therefore absent,
	// unchanged, or gone only if it is one the harness itself left stale by
	// killing the upgrader (cjsiAccKnownStaleLocks); a lock that is newly
	// present, rewritten, or any other lock that disappeared is a write.
	cjsiAccLock
	// scan-receipts.json: existing receipts are immutable; a new receipt
	// must be a terminal failed round of this fixture (cjsiAccReceiptDiffs).
	cjsiAccReceipts
	// Architecture §7.1: the old opener may create EMPTY sidecars under the
	// fixed legacy (fence) name; they cannot make it a database.
	cjsiAccEmptyFenceSidecar
	// An EMPTY 0600 core rollback journal: the old build's core opener
	// leaves a zero-length agentdeck.sqlite3-journal (observed after the old
	// `session scan` on an interrupted layout, and on the old build's own
	// schema-35 roots). A zero-length journal is never hot and cannot roll
	// anything back; the core's content stays governed by its logical digest.
	cjsiAccEmptyCoreJournal
)

func cjsiAccRuleFor(name string) cjsiAccRule {
	switch name {
	case cjsiAccCoreName:
		return cjsiAccCoreRule
	case cjsiAccCoreName + "-wal":
		return cjsiAccCoreWAL
	case cjsiAccCoreName + "-shm":
		return cjsiAccCoreSHM
	case cjsiAccCoreName + "-journal":
		return cjsiAccEmptyCoreJournal
	case "state.lock", "scan.lock":
		return cjsiAccLock
	case "scan-receipts.json":
		return cjsiAccReceipts
	case "sessions.sqlite3-shm", "sessions.sqlite3-wal", "sessions.sqlite3-journal":
		return cjsiAccEmptyFenceSidecar
	}
	return cjsiAccExact
}

var cjsiAccLockToken = regexp.MustCompile(`^v1:([1-9][0-9]*):[0-9a-f]{32}$`)

// cjsiAccSideEffects lists every difference between two snapshots that the
// rules above do not allow.
func cjsiAccSideEffects(before, after cjsiAccSnapshot) []string {
	var diffs []string
	check := func(name string, b, a cjsiAccFile, hadBefore, hasAfter bool) {
		switch cjsiAccRuleFor(name) {
		case cjsiAccExact:
			switch {
			case hadBefore && !hasAfter:
				diffs = append(diffs, "removed "+name)
			case !hadBefore && hasAfter:
				diffs = append(diffs, fmt.Sprintf("created %s (%d bytes)", name, a.size))
			case a.ino != b.ino:
				diffs = append(diffs, "replaced "+name)
			case a.mode != b.mode:
				diffs = append(diffs, fmt.Sprintf("mode %s %v -> %v", name, b.mode, a.mode))
			case a.sha != b.sha || a.size != b.size:
				diffs = append(diffs, fmt.Sprintf("rewrote %s (%d -> %d bytes)", name, b.size, a.size))
			}
		case cjsiAccCoreRule:
			switch {
			case hadBefore != hasAfter:
				diffs = append(diffs, "core presence changed")
			case a.ino != b.ino || a.mode != b.mode:
				diffs = append(diffs, "replaced or chmod "+name)
			case a.logical != b.logical:
				diffs = append(diffs, fmt.Sprintf("logically rewrote %s (%d -> %d bytes)", name, b.size, a.size))
			}
		case cjsiAccCoreWAL:
			if hasAfter && a.mode != 0o600 {
				diffs = append(diffs, fmt.Sprintf("core WAL %s is %v", name, a.mode))
			}
		case cjsiAccCoreSHM:
			if hasAfter && a.mode != 0o600 {
				diffs = append(diffs, fmt.Sprintf("core wal-index %s is %v", name, a.mode))
			}
		case cjsiAccLock:
			switch {
			case hasAfter && !a.mode.IsRegular():
				diffs = append(diffs, fmt.Sprintf("lock %s is %v, not a regular file", name, a.mode.Type()))
			case !hadBefore && hasAfter:
				diffs = append(diffs, fmt.Sprintf("lock %s held at a quiescent point: %q", name, cjsiAccClip(string(a.content))))
			case hadBefore && !hasAfter:
				if _, known := cjsiAccKnownStaleLocks.Load(cjsiAccLockKey(b.path, b.ino, b.content)); !known {
					diffs = append(diffs, fmt.Sprintf("lock %s removed although the harness did not leave it stale: %q", name, cjsiAccClip(string(b.content))))
				}
			case hadBefore && hasAfter:
				if a.ino != b.ino || a.mode != b.mode || a.sha != b.sha {
					diffs = append(diffs, fmt.Sprintf("lock %s changed at a quiescent point: %q -> %q", name, cjsiAccClip(string(b.content)), cjsiAccClip(string(a.content))))
				}
			}
		case cjsiAccReceipts:
			// The scan runtime replaces the journal only by writing a
			// temporary and renaming it over the old one, and only to
			// change its content. So the same inode must keep the same
			// bytes, and a new inode must carry new content; deleting and
			// rebuilding the same bytes (new inode, same digest) or
			// rewriting in place is a write by someone else.
			switch {
			case hadBefore && !hasAfter:
				diffs = append(diffs, "removed "+name)
			case hasAfter && a.mode != 0o600:
				diffs = append(diffs, fmt.Sprintf("receipt journal is %v", a.mode))
			case hadBefore && a.ino == b.ino && a.sha != b.sha:
				diffs = append(diffs, "receipt journal rewritten in place")
			case hadBefore && a.ino != b.ino && a.sha == b.sha:
				diffs = append(diffs, "receipt journal deleted and rebuilt with the same content (new inode)")
			case hasAfter:
				diffs = append(diffs, cjsiAccReceiptDiffs(b.content, a.content, a.home)...)
			}
		case cjsiAccEmptyFenceSidecar:
			if hasAfter && (!a.mode.IsRegular() || a.size != 0) {
				diffs = append(diffs, fmt.Sprintf("non-empty sidecar %s (%d bytes)", name, a.size))
			}
		case cjsiAccEmptyCoreJournal:
			if hasAfter && (a.mode != 0o600 || a.size != 0) {
				diffs = append(diffs, fmt.Sprintf("core journal %s is %v with %d bytes", name, a.mode, a.size))
			}
		}
	}
	for _, name := range before.names() {
		a, ok := after[name]
		check(name, before[name], a, true, ok)
	}
	for _, name := range after.names() {
		if _, ok := before[name]; !ok {
			check(name, cjsiAccFile{}, after[name], false, true)
		}
	}
	return diffs
}

// The receipt journal shape written by the scan runtime (both builds). Every
// object is decoded with unknown fields refused.
type cjsiAccReceiptDomain struct {
	OwnershipRefusal json.RawMessage `json:"ownership_refusal,omitempty"`
	State            string          `json:"state"`
	Changes          map[string]int  `json:"changes,omitempty"`
	Scan             *struct {
		Sources   int `json:"sources"`
		Documents int `json:"documents"`
		Skipped   int `json:"skipped"`
		Removed   int `json:"removed"`
	} `json:"scan,omitempty"`
	ErrorCode  string `json:"error_code,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

type cjsiAccReceipt struct {
	ID            string  `json:"id"`
	StateID       string  `json:"state_id"`
	Home          string  `json:"home"`
	Scope         string  `json:"scope"`
	RoundID       string  `json:"round_id"`
	Observation   uint64  `json:"observation"`
	State         string  `json:"state"`
	RoundTerminal bool    `json:"round_terminal"`
	AcceptedAt    string  `json:"accepted_at"`
	TerminalAt    *string `json:"terminal_at,omitempty"`
	Result        *struct {
		RoundID     string                 `json:"round_id"`
		StartedAt   string                 `json:"started_at"`
		CompletedAt string                 `json:"completed_at"`
		Usage       cjsiAccReceiptDomain   `json:"usage"`
		Session     cjsiAccReceiptDomain   `json:"session"`
		Stages      map[string]json.Number `json:"stages"`
	} `json:"result,omitempty"`
	AppOwned bool `json:"app_owned,omitempty"`
}

// cjsiAccStrictJSON decodes exactly one JSON value, refusing unknown fields
// and anything but whitespace after it. The tail check is a second Decode
// that must report io.EOF: Decoder.More only reports whether another array
// element or object member could follow, so it accepts a stray ']' or '}'
// that the production reader (json.Unmarshal) rejects.
func cjsiAccStrictJSON(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing data: a second JSON value follows")
		}
		return fmt.Errorf("trailing data: %w", err)
	}
	return nil
}

// cjsiAccParseReceipts decodes the journal strictly and returns each receipt
// with its canonical raw bytes, keyed by id.
func cjsiAccParseReceipts(content []byte) (map[string]cjsiAccReceipt, map[string]string, error) {
	var journal struct {
		Version int               `json:"version"`
		Entries []json.RawMessage `json:"entries"`
	}
	if err := cjsiAccStrictJSON(content, &journal); err != nil {
		return nil, nil, fmt.Errorf("receipt journal: %w", err)
	}
	if journal.Version != 1 {
		return nil, nil, fmt.Errorf("receipt journal version %d", journal.Version)
	}
	receipts, raw := map[string]cjsiAccReceipt{}, map[string]string{}
	for _, entry := range journal.Entries {
		var receipt cjsiAccReceipt
		if err := cjsiAccStrictJSON(entry, &receipt); err != nil {
			return nil, nil, fmt.Errorf("receipt entry: %w", err)
		}
		if _, dup := receipts[receipt.ID]; dup || receipt.ID == "" {
			return nil, nil, fmt.Errorf("receipt id %q duplicated or empty", receipt.ID)
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, entry); err != nil {
			return nil, nil, err
		}
		receipts[receipt.ID], raw[receipt.ID] = receipt, compact.String()
	}
	return receipts, raw, nil
}

// cjsiAccFailedDomain accepts only a domain that did nothing: the fenced
// layout makes an old usage round stop on schema_ahead and an old session
// round fail before opening the index; neither reports changes or scanned
// sources, and neither claims an ownership refusal.
func cjsiAccFailedDomain(domain cjsiAccReceiptDomain) bool {
	if len(domain.OwnershipRefusal) != 0 || len(domain.Changes) != 0 {
		return false
	}
	if domain.Scan != nil && (domain.Scan.Sources != 0 || domain.Scan.Documents != 0 || domain.Scan.Skipped != 0 || domain.Scan.Removed != 0) {
		return false
	}
	return domain.State == "failed" && (domain.ErrorCode == "schema_ahead" || domain.ErrorCode == "scan_failed")
}

// cjsiAccReceiptDiffs validates the journal change between two snapshots:
// existing receipts are immutable and never removed; each new receipt is a
// terminal, non-app round of this fixture's home in which every domain
// failed without effect.
func cjsiAccReceiptDiffs(before, after []byte, home string) []string {
	var old map[string]string
	if before != nil {
		var err error
		if _, old, err = cjsiAccParseReceipts(before); err != nil {
			return []string{"before: " + err.Error()}
		}
	}
	receipts, raw, err := cjsiAccParseReceipts(after)
	if err != nil {
		return []string{err.Error()}
	}
	var diffs []string
	for id, text := range old {
		if raw[id] != text {
			diffs = append(diffs, "receipt "+id+" changed or removed")
		}
	}
	for id, receipt := range receipts {
		if _, existed := old[id]; existed {
			continue
		}
		switch {
		case receipt.State != "terminal" || !receipt.RoundTerminal || receipt.TerminalAt == nil || receipt.AppOwned:
			diffs = append(diffs, fmt.Sprintf("receipt %s is not a terminal CLI round: %s", id, raw[id]))
		case receipt.Home != home:
			diffs = append(diffs, fmt.Sprintf("receipt %s names home %q, not the fixture's", id, receipt.Home))
		case receipt.Scope != "both" && receipt.Scope != "usage" && receipt.Scope != "session":
			diffs = append(diffs, fmt.Sprintf("receipt %s has scope %q", id, receipt.Scope))
		case receipt.Result == nil || receipt.Result.RoundID != receipt.RoundID:
			diffs = append(diffs, fmt.Sprintf("receipt %s lacks its round result: %s", id, raw[id]))
		case !cjsiAccFailedDomain(receipt.Result.Usage) || !cjsiAccFailedDomain(receipt.Result.Session):
			diffs = append(diffs, fmt.Sprintf("receipt %s records a domain outcome other than a no-op failure: %s", id, cjsiAccClip(raw[id])))
		}
	}
	sort.Strings(diffs)
	return diffs
}

// cjsiAccAssertReceipts validates the whole receipt journal of a fenced root
// as if it had been empty before: every receipt is a no-op failed round.
func cjsiAccAssertReceipts(t *testing.T, state string) {
	t.Helper()
	content, _, err := cjsiAccReadRegular(filepath.Join(state, "scan-receipts.json"), cjsiAccStateReadLimit)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if diffs := cjsiAccReceiptDiffs(nil, content, filepath.Join(filepath.Dir(state), "home")); len(diffs) > 0 {
		t.Fatalf("scan receipts on fenced state: %v", diffs)
	}
}

// cjsiAccUpgrader builds testdata/cjsi_acceptance/upgrader once per test run.
// The overlay maps store_upgrade.go into internal/store only to reach the
// committed, hookable migration entry: complete mode is the production
// store.MigrateState, interrupt mode is migrateCJSIManagedRootWithReady with
// the pause in its beforeReady hook. The harness adds no migration step.
type cjsiAccUpgrader struct {
	dir  string // owned by the top-level test, so the binary outlives subtests
	once sync.Once
	path string
	err  error
}

func (u *cjsiAccUpgrader) binary(t *testing.T) string {
	t.Helper()
	u.once.Do(func() { u.path, u.err = cjsiAccBuildUpgrader(u.dir) })
	if u.err != nil {
		t.Fatalf("build CJSI acceptance upgrader: %v", u.err)
	}
	return u.path
}

func cjsiAccBuildUpgrader(dir string) (string, error) {
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(module, "go.mod")); err != nil {
		return "", fmt.Errorf("module root: %w", err)
	}
	fixture := filepath.Join(module, "cmd", "agentdeck", "testdata", "cjsi_acceptance", "store_upgrade.go")
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": {
		filepath.Join(module, "internal", "store", "cjsi_acceptance_upgrade.go"): fixture,
	}})
	if err != nil {
		return "", err
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o600); err != nil {
		return "", err
	}
	goBinary, err := exec.LookPath("go")
	if err != nil {
		return "", err
	}
	out := filepath.Join(dir, "cjsi-acceptance-upgrader")
	ctx, cancel := context.WithTimeout(context.Background(), cjsiAccBuildGuard)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBinary, "build", "-mod=vendor", "-tags", "cjsi_acceptance_upgrade", "-overlay", overlayPath, "-o", out, "./cmd/agentdeck/testdata/cjsi_acceptance/upgrader")
	cmd.Dir = module
	// The toolchain keeps the caller's Go cache settings; the program it
	// builds runs later under the fixture environment only.
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=vendor", "GOPROXY=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%v\n%s", err, output)
	}
	return out, nil
}

// upgrade runs the production managed migration (store.MigrateState) to
// completion.
func (r *cjsiAccRoot) upgrade(t *testing.T, upgrader string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), cjsiAccCommandGuard)
	defer cancel()
	cmd := cjsiAccGuarded(ctx, t, upgrader, r.state, "complete")
	cmd.Env = r.env()
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "CJSI-UPGRADE-COMPLETE") {
		t.Fatalf("managed upgrade: %v\n%s", err, output)
	}
}

// upgradeInterrupted starts the managed migration, waits for its beforeReady
// hook to report (fence and index verified, ready journal and initial head not
// yet committed), and kills that process
// only. The pause is a handshake on the reported line, not a delay.
func (r *cjsiAccRoot) upgradeInterrupted(t *testing.T, upgrader string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), cjsiAccCommandGuard)
	defer cancel()
	cmd := cjsiAccGuarded(ctx, t, upgrader, r.state, "pause-before-ready")
	cmd.Env = r.env()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, readErr := bufio.NewReader(stdout).ReadString('\n')
	if strings.TrimSpace(line) != "CJSI-UPGRADE-PAUSED" {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("upgrader did not reach the pause: %q %v %s", line, readErr, stderr.String())
	}
	if err := cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, stdout)
	if err := cmd.Wait(); err == nil {
		t.Fatal("killed upgrader reported success")
	}
	// sandbox-exec execs the upgrader in place, so its pid is the one in the
	// lock tokens it left.
	r.recordUpgraderLocks(t, cmd.Process.Pid)
}

func cjsiAccExternalBinary(t *testing.T, name string) string {
	t.Helper()
	path := os.Getenv(name)
	if path == "" {
		t.Skipf("external-binary submatrix skipped: %s is unset (tasks.md permits skipping it in ordinary ./... runs; the combined L3 command must set it)", name)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%s=%q is not an executable file: %v", name, path, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s sha256=%x", name, sha256.Sum256(content))
	return path
}

func cjsiAccPending(t *testing.T, dependency, needs string) {
	t.Helper()
	t.Skipf("PENDING(%s): %s", dependency, needs)
}
