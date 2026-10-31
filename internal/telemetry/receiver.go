package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/kitdine/agent-deck/internal/perftrace"
	"github.com/kitdine/agent-deck/internal/store"
)

// Receiver answers OTLP/HTTP JSON log exports. It acknowledges a request only
// after its allowlisted records are fsync'd to the journal; it never answers
// success before the write.
type Receiver struct {
	Token   string
	Journal *Journal
	// Persisted is signalled, without blocking, after each durable append.
	Persisted chan<- struct{}
	// Refused records a request that could not be persisted.
	Refused func(at time.Time, reason string)
	Now     func() time.Time
}

func (r *Receiver) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	if request.URL.Path != LogsPath {
		http.NotFound(w, request)
		return
	}
	if request.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeStatus(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	if !tokenMatches(r.Token, request.Header.Get(TokenHeader)) {
		writeStatus(w, http.StatusUnauthorized, "token_invalid")
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		// Only OTLP/HTTP JSON is supported; protobuf is not silently accepted.
		writeStatus(w, http.StatusUnsupportedMediaType, "json_required")
		return
	}
	if request.Header.Get("Content-Encoding") != "" && request.Header.Get("Content-Encoding") != "identity" {
		writeStatus(w, http.StatusUnsupportedMediaType, "encoding_unsupported")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, request.Body, MaxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeStatus(w, http.StatusRequestEntityTooLarge, "body_too_large")
			return
		}
		writeStatus(w, http.StatusBadRequest, "body_unreadable")
		return
	}
	records, err := Decode(body)
	if err != nil {
		writeStatus(w, http.StatusBadRequest, "malformed")
		return
	}
	if err := r.Journal.Append(records); err != nil {
		reason := "journal_write_failed"
		if errors.Is(err, ErrJournalFull) {
			reason = "journal_full"
		}
		if r.Refused != nil {
			r.Refused(r.Now(), reason)
		}
		writeStatus(w, http.StatusServiceUnavailable, reason)
		return
	}
	perftrace.Mark(perftrace.OTelAck, perftrace.Count{Key: "records", Value: int64(len(records))})
	if len(records) > 0 && r.Persisted != nil {
		select {
		case r.Persisted <- struct{}{}:
		default:
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "{}")
}

func writeStatus(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// ListenerState is the running listener's self-description, written by the
// listener process. It never contains the token.
type ListenerState struct {
	PID       int    `json:"pid"`
	Owner     string `json:"owner"`
	Port      int    `json:"port"`
	State     string `json:"state"`
	Error     string `json:"error,omitempty"`
	StartedAt string `json:"started_at"`
	// LastRefusedAt is the latest request the listener could not persist.
	LastRefusedAt string `json:"last_refused_at,omitempty"`
	LastRefusal   string `json:"last_refusal,omitempty"`
}

func listenerPath(stateRoot string) string { return filepath.Join(Dir(stateRoot), "listener.json") }

func writeListenerState(stateRoot string, state ListenerState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return writePrivateFile(listenerPath(stateRoot), append(data, '\n'))
}

// ReadListenerState returns the listener's last self-description. A running
// state whose process no longer exists is reported as stopped.
func ReadListenerState(stateRoot string) (ListenerState, error) {
	data, err := os.ReadFile(listenerPath(stateRoot))
	if errors.Is(err, os.ErrNotExist) {
		return ListenerState{State: "stopped"}, nil
	}
	if err != nil {
		return ListenerState{}, err
	}
	var state ListenerState
	if err := json.Unmarshal(data, &state); err != nil {
		return ListenerState{State: "unknown"}, nil
	}
	if state.State == "running" && !processAlive(state.PID) {
		return ListenerState{State: "stopped"}, nil
	}
	return state, nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// ServeOptions configure one listener lifetime.
type ServeOptions struct {
	StateRoot string
	// Home locates the user-level Codex configuration for the linkage.
	Home  string
	Owner string
	Store *store.Store
	Now   func() time.Time
	// Ready, when set, is called once the listener is bound.
	Ready func(addr string)
	// Published requests an owner refresh after counted usage changes or a
	// finite inventory finishes (which may already have settled and published).
	Published func()
	// Inventory requests one finite reconciliation when pending observations
	// lack a post-receipt inventory. It runs independently of HTTP reception.
	Inventory func(context.Context) error
	// SettleEvery is the interval between settle passes; zero uses one minute.
	SettleEvery time.Duration
}

var (
	// ErrDisabled reports that the receiver is not enabled.
	ErrDisabled = errors.New("telemetry receiver is disabled")
	// ErrCollectionNotNeeded reports that the linkage keeps the receiver
	// stopped: Codex Memories is explicitly off.
	ErrCollectionNotNeeded = errors.New("codex memories is off; collection is not needed")
)

// Serve binds the loopback listener and runs until ctx ends or the receiver
// is disabled. It records the listening interval and refusal gaps, and
// drains the journal at start (recovering after a crash) and after every
// acknowledged request.
func Serve(ctx context.Context, options ServeOptions) error {
	// Capture baselines before reading the settings or announcing readiness.
	// A startup/Ready callback change must still be noticed by the first poll.
	configStamp := configModTime(options.StateRoot)
	codexStamp := fileModTime(CodexConfigPath(options.Home))
	config, err := LoadConfig(options.StateRoot)
	if err != nil {
		return err
	}
	if !config.Enabled {
		return ErrDisabled
	}
	if ReadCodexFacts(options.Home, config.Port).CollectionBlocked() {
		_ = os.Remove(listenerPath(options.StateRoot))
		return ErrCollectionNotNeeded
	}
	if err := ensureDir(options.StateRoot); err != nil {
		return err
	}
	token, err := EnsureToken(options.StateRoot)
	if err != nil {
		return err
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	started := now()
	state := ListenerState{PID: os.Getpid(), Owner: options.Owner, Port: config.Port, StartedAt: started.UTC().Format(time.RFC3339)}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(config.Port)))
	if err != nil {
		state.State, state.Error = "failed", "bind_failed"
		_ = writeListenerState(options.StateRoot, state)
		return err
	}
	defer listener.Close()
	journal := &Journal{
		Dir: JournalDir(options.StateRoot), SegmentBytes: DefaultSegmentBytes, Cap: DefaultJournalCap,
		Retention: time.Duration(config.RetentionDays) * 24 * time.Hour,
		Processed: func() (store.TelemetryCursor, error) {
			return options.Store.TelemetryCursorPosition(context.Background())
		},
		Now: now,
	}
	interval, err := options.Store.BeginTelemetryListening(ctx, started)
	if err != nil {
		return err
	}
	persisted := make(chan struct{}, 1)
	refusals := make(chan [2]string, 16)
	receiver := &Receiver{
		Token: token, Journal: journal, Persisted: persisted, Now: now,
		Refused: func(at time.Time, reason string) {
			select {
			case refusals <- [2]string{at.UTC().Format(time.RFC3339Nano), reason}:
			default:
			}
		},
	}
	server := &http.Server{Handler: receiver, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	state.State = "running"
	if err := writeListenerState(options.StateRoot, state); err != nil {
		_ = server.Close()
		return err
	}
	if options.Ready != nil {
		options.Ready(listener.Addr().String())
	}
	reconciler := &Reconciler{Store: options.Store, Journal: journal, Now: now, Batch: 500,
		SessionsChanged: func(ctx context.Context) error {
			index, err := store.OpenSessions(ctx, options.StateRoot)
			if err != nil {
				return err
			}
			defer index.Close()
			return ProjectSessionIndex(ctx, options.Store, index)
		}}
	settle := func() bool {
		if result, err := reconciler.Settle(context.Background()); err == nil && result.Changed {
			if options.Published != nil {
				options.Published()
			}
			return true
		}
		return false
	}
	inventoryDone := make(chan error, 1)
	inventoryRunning := false
	requestInventory := func() {
		if options.Inventory == nil || inventoryRunning || ctx.Err() != nil {
			return
		}
		needed, err := options.Store.TelemetryNeedsInventory(ctx)
		if err != nil || !needed {
			return
		}
		inventoryRunning = true
		go func() { inventoryDone <- options.Inventory(ctx) }()
	}
	drain := func() {
		if result, err := reconciler.Drain(context.Background()); err == nil {
			_ = journal.Cleanup()
			if result.Changed {
				settle()
			}
			requestInventory()
		}
	}
	drain()
	settle()
	settleEvery := options.SettleEvery
	if settleEvery <= 0 {
		settleEvery = time.Minute
	}
	settleTicker := time.NewTicker(settleEvery)
	defer settleTicker.Stop()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	exitReason := "stopped"
	var serveErr error
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case serveErr = <-served:
			exitReason = "listener_failed"
			break loop
		case <-persisted:
			drain()
		case refusal := <-refusals:
			at, _ := time.Parse(time.RFC3339Nano, refusal[0])
			_ = options.Store.RecordTelemetryGap(context.Background(), at, at, refusal[1])
			state.LastRefusedAt, state.LastRefusal = refusal[0], refusal[1]
			_ = writeListenerState(options.StateRoot, state)
		case <-settleTicker.C:
			settle()
			requestInventory()
		case err := <-inventoryDone:
			inventoryRunning = false
			if changed := settle(); err == nil && !changed && options.Published != nil {
				// The scan can settle an old pending observation itself. Its
				// publication still needs to reach an owner with no source event.
				options.Published()
			}
		case <-ticker.C:
			// The configuration is AgentDeck's own file; a disable from the
			// CLI or Settings stops this listener without a signal.
			if stamp := configModTime(options.StateRoot); !stamp.Equal(configStamp) {
				configStamp = stamp
				if current, err := LoadConfig(options.StateRoot); err == nil && !current.Enabled {
					exitReason = "disabled"
					break loop
				}
			}
			// Turning Codex Memories off stops collection; the preference
			// stays, and the App starts the listener again when it is on.
			if stamp := fileModTime(CodexConfigPath(options.Home)); !stamp.Equal(codexStamp) {
				codexStamp = stamp
				if ReadCodexFacts(options.Home, config.Port).CollectionBlocked() {
					exitReason = ReasonMemoriesOff
					break loop
				}
			}
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
	drain()
	settle()
	_ = options.Store.EndTelemetryListening(context.Background(), interval, now(), exitReason)
	_ = os.Remove(listenerPath(options.StateRoot))
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	return nil
}

func configModTime(stateRoot string) time.Time { return fileModTime(configPath(stateRoot)) }

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
