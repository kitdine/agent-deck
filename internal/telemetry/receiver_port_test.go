package telemetry

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestServeMovesToAChangedPortAndKeepsReceivingUntilItCan(t *testing.T) {
	database, root := openStore(t)
	home := t.TempDir()
	writeCodexConfig(t, home, "[features]\nmemories = true\n", 0o600)
	first, second := freePort(t), freePort(t)
	if err := SaveConfig(root, Config{Enabled: true, Port: first, RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	// Something else holds the requested port at first.
	blocker, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(second)))
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, ServeOptions{StateRoot: root, Home: home, Owner: "foreground", Store: database, Ready: func(addr string) { ready <- addr }})
	}()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("receiver never became ready")
	}
	if err := SaveConfig(root, Config{Enabled: true, Port: second, RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	waitForListener(t, root, func(state ListenerState) bool { return state.Error == "bind_failed" })
	if state, _ := ReadListenerState(root); state.Port != first || !answers(first) {
		t.Fatalf("refused port stopped reception: %+v", state)
	}
	if err := blocker.Close(); err != nil {
		t.Fatal(err)
	}
	waitForListener(t, root, func(state ListenerState) bool { return state.Port == second && state.Error == "" })
	if !answers(second) {
		t.Fatal("receiver does not answer on the changed port")
	}
	if answers(first) {
		t.Fatal("receiver still answers on the previous port")
	}
}

func waitForListener(t *testing.T, root string, want func(ListenerState) bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		state, err := ReadListenerState(root)
		if err == nil && want(state) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("listener state = %+v err=%v", state, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func answers(port int) bool {
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Post("http://127.0.0.1:"+strconv.Itoa(port)+"/v1/logs", "application/json", strings.NewReader("{}"))
	if err != nil {
		return false
	}
	response.Body.Close()
	return true
}
