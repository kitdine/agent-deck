package store

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func cjsiSlotTestRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	return root
}

func checkCJSISlotBytes(t *testing.T, path string, want []byte) os.FileInfo {
	t.Helper()
	st, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode() != 0600 || st.Size() != int64(len(want)) {
		t.Fatalf("unexpected slot mode %s or size %d", st.Mode(), st.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, want) {
		t.Fatalf("incorrect slot contents (read error %v)", err)
	}
	return st
}

// Keep an unrelated staging file in the directory throughout to check
// that failures preserve entries the caller did not authorize replacing.
func cjsiSlotTestEntries(t *testing.T, root string) []string {
	t.Helper()
	const neighbor = ".cjsi-serving-tmp-neighbor"
	checkCJSISlotBytes(t, filepath.Join(root, neighbor), []byte("neighbor"))
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if entry.Name() != neighbor {
			names = append(names, entry.Name())
		}
	}
	return names
}

func TestCJSISlotDurability(t *testing.T) {
	// This identity comes from the synthetic candidate. The slot writer never
	// generates a substitute for it, including when an earlier attempt failed.
	identity := [16]byte{0x12, 0x34, 15: 0xab}
	const name = ".cjsi-serving-slot-123400000000000000000000000000ab"
	payload := []byte("prepared\x00slot\n")
	injected := errors.New("injected sync error")
	for _, tc := range []struct {
		name   string
		failAt int
	}{
		{"both-syncs", 0},
		{"file-sync-failure", 1},
		{"directory-sync-failure", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := cjsiSlotTestRoot(t)
			if err := os.WriteFile(filepath.Join(root, ".cjsi-serving-tmp-neighbor"), []byte("neighbor"), 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, name)
			type result struct {
				slot cjsiDurableSlot
				err  error
			}
			entered := make(chan int)
			release := make(chan error)
			stop := make(chan struct{})
			finished := make(chan struct{})
			done := make(chan result, 1)
			go func() {
				defer close(finished)
				slot, err := persistCJSIServingSlotWithSync(root, identity, payload, func(fd int) error {
					select {
					case entered <- fd:
					case <-stop:
						return injected
					}
					select {
					case err := <-release:
						if err != nil {
							return err
						}
						return unix.Fsync(fd)
					case <-stop:
						return injected
					}
				})
				done <- result{slot, err}
			}()
			defer func() { close(stop); <-finished }()
			var staged os.FileInfo
			for phase := 1; phase <= 2; phase++ {
				var fd int
				select {
				case fd = <-entered:
				case got := <-done:
					t.Fatalf("returned before sync %d: %+v", phase, got)
				}
				var st unix.Stat_t
				if err := unix.Fstat(fd, &st); err != nil {
					t.Fatal(err)
				}
				names := cjsiSlotTestEntries(t, root)
				if len(names) != 1 {
					t.Fatalf("unexpected files at sync %d: %v", phase, names)
				}
				info := checkCJSISlotBytes(t, filepath.Join(root, names[0]), payload)
				var named unix.Stat_t
				if phase == 1 {
					if !strings.HasPrefix(names[0], ".cjsi-serving-tmp-") || st.Mode&unix.S_IFMT != unix.S_IFREG {
						t.Fatalf("expected staged file at first sync, got %v, mode %o", names, st.Mode)
					}
					staged = info
					if err := unix.Lstat(filepath.Join(root, names[0]), &named); err != nil {
						t.Fatal(err)
					}
				} else {
					if names[0] != name || st.Mode&unix.S_IFMT != unix.S_IFDIR || !os.SameFile(staged, info) {
						t.Fatalf("expected exclusive install of staged inode at directory sync, got %v, mode %o", names, st.Mode)
					}
					if err := unix.Lstat(root, &named); err != nil {
						t.Fatal(err)
					}
				}
				if st.Dev != named.Dev || st.Ino != named.Ino {
					t.Fatalf("sync %d on wrong descriptor", phase)
				}
				select {
				case got := <-done:
					t.Fatalf("returned while sync %d paused: %+v", phase, got)
				default:
				}
				if phase == tc.failAt {
					release <- injected
					break
				}
				release <- nil
			}
			var got result
			select {
			case got = <-done:
			case <-entered:
				t.Fatal("unexpected additional sync")
			}
			if tc.failAt != 0 {
				if !errors.Is(got.err, injected) || got.slot != (cjsiDurableSlot{}) {
					t.Fatalf("failed sync returned %+v", got)
				}
			} else if got.err != nil || got.slot != (cjsiDurableSlot{identity, sha256.Sum256(payload), int64(len(payload))}) {
				t.Fatalf("incorrect durable result: %+v", got)
			}
			if tc.failAt == 1 {
				names := cjsiSlotTestEntries(t, root)
				if len(names) != 1 || !strings.HasPrefix(names[0], ".cjsi-serving-tmp-") {
					t.Fatalf("file sync failure should retain one unsealed staging entry, got %v", names)
				}
				if !os.SameFile(staged, checkCJSISlotBytes(t, filepath.Join(root, names[0]), payload)) {
					t.Fatal("file sync failure changed the staged inode")
				}
				return
			}
			slot, err := persistCJSIServingSlot(root, identity, []byte("different retry"))
			if !errors.Is(err, os.ErrExist) || slot != (cjsiDurableSlot{}) {
				t.Fatalf("retry collision returned %+v, %v", slot, err)
			}
			if names := cjsiSlotTestEntries(t, root); len(names) != 1 || names[0] != name || !os.SameFile(staged, checkCJSISlotBytes(t, path, payload)) {
				t.Fatalf("retry changed the installed slot or leaked a temp file: %v", names)
			}
		})
	}

	t.Run("admission-and-limits", func(t *testing.T) {
		root := cjsiSlotTestRoot(t)
		link := filepath.Join(filepath.Dir(root), "linked-root")
		if err := os.Symlink(root, link); err != nil {
			t.Fatal(err)
		}
		fail := func(dir string, id [16]byte, data []byte, want error) {
			t.Helper()
			got, err := persistCJSIServingSlot(dir, id, data)
			if got != (cjsiDurableSlot{}) || err == nil || (want != nil && !errors.Is(err, want)) {
				t.Fatalf("unexpected admission result %+v, %v", got, err)
			}
		}
		fail(link, identity, payload, nil)
		fail(link+"/", identity, payload, errCJSISlotAdmission)
		fail(filepath.Join(root, "absent"), identity, payload, os.ErrNotExist)
		fail(".", identity, payload, errCJSISlotAdmission)
		fail(root, [16]byte{}, payload, errCJSISlotAdmission)
		fail(root, identity, nil, errCJSISlotAdmission)
		atLimit := make([]byte, cjsiMaxSlotBytes+1)
		fail(root, identity, atLimit, errCJSISlotTooLarge)
		if err := os.Chmod(root, 0755); err != nil {
			t.Fatal(err)
		}
		fail(root, identity, payload, errCJSISlotAdmission)
		if err := os.Chmod(root, 0700); err != nil {
			t.Fatal(err)
		}
		if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
			t.Fatalf("refused attempts left entries: %v, %v", entries, err)
		}
		data := atLimit[:cjsiMaxSlotBytes]
		slot, err := persistCJSIServingSlot(root, identity, data)
		if err != nil || slot != (cjsiDurableSlot{identity, sha256.Sum256(data), cjsiMaxSlotBytes}) {
			t.Fatalf("maximum-size slot returned %+v, %v", slot, err)
		}
		checkCJSISlotBytes(t, filepath.Join(root, name), data)
	})

	t.Run("collision-after-staging", func(t *testing.T) {
		root := cjsiSlotTestRoot(t)
		path := filepath.Join(root, name)
		other := []byte("winning write")
		var winner os.FileInfo
		slot, err := persistCJSIServingSlotWithSync(root, identity, payload, func(fd int) error {
			if winner != nil {
				t.Fatal("losing write reached directory sync")
			}
			if err := unix.Fsync(fd); err != nil {
				t.Fatal(err)
			}
			// Force a winner after the loser's file sync and before its install.
			got, err := persistCJSIServingSlot(root, identity, other)
			if err != nil || got != (cjsiDurableSlot{identity, sha256.Sum256(other), int64(len(other))}) {
				t.Fatalf("competing install: %+v, %v", got, err)
			}
			winner = checkCJSISlotBytes(t, path, other)
			return nil
		})
		if winner == nil || slot != (cjsiDurableSlot{}) || !errors.Is(err, os.ErrExist) {
			t.Fatalf("late collision returned %+v, %v", slot, err)
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 2 || entries[0].Name() != name ||
			!strings.HasPrefix(entries[1].Name(), ".cjsi-serving-tmp-") || !os.SameFile(winner, checkCJSISlotBytes(t, path, other)) {
			t.Fatalf("late collision must preserve the winner and retain one unsealed staging entry: %v, %v", entries, err)
		}
		checkCJSISlotBytes(t, filepath.Join(root, entries[1].Name()), payload)
	})

	t.Run("occupied-symlink", func(t *testing.T) {
		root := cjsiSlotTestRoot(t)
		path := filepath.Join(root, name)
		target := filepath.Join(filepath.Dir(root), "absent")
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		slot, err := persistCJSIServingSlot(root, identity, payload)
		if slot != (cjsiDurableSlot{}) || !errors.Is(err, os.ErrExist) {
			t.Fatalf("occupied slot returned %+v, %v", slot, err)
		}
		if dest, err := os.Readlink(path); err != nil || dest != target {
			t.Fatalf("existing link changed: %q, %v", dest, err)
		}
		if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("symlink target touched: %v", err)
		}
		if entries, err := os.ReadDir(root); err != nil || len(entries) != 1 || entries[0].Name() != name {
			t.Fatalf("an already occupied slot should be refused before staging: %v, %v", entries, err)
		}
	})

	for _, fault := range []string{"staging-name-rebound", "installed-inode-rebound", "installed-content-changed", "installed-mode-changed"} {
		t.Run(fault, func(t *testing.T) {
			root := cjsiSlotTestRoot(t)
			path := filepath.Join(root, name)
			slot, err := persistCJSIServingSlotWithSync(root, identity, payload, func(fd int) error {
				if err := unix.Fsync(fd); err != nil {
					t.Fatal(err)
				}
				var st unix.Stat_t
				if err := unix.Fstat(fd, &st); err != nil {
					t.Fatal(err)
				}
				switch {
				case fault == "staging-name-rebound" && st.Mode&unix.S_IFMT == unix.S_IFREG:
					entries, err := os.ReadDir(root)
					if err != nil || len(entries) != 1 {
						t.Fatalf("staging file: %v, %v", entries, err)
					}
					path = filepath.Join(root, entries[0].Name())
				case fault == "staging-name-rebound" || st.Mode&unix.S_IFMT != unix.S_IFDIR:
					return nil
				}
				switch fault {
				case "installed-content-changed":
					return os.WriteFile(path, bytes.Repeat([]byte{'x'}, len(payload)), 0600)
				case "installed-mode-changed":
					return os.Chmod(path, 0644)
				default:
					if err := os.Rename(path, filepath.Join(root, "original-inode")); err != nil {
						t.Fatal(err)
					}
					return os.WriteFile(path, payload, 0600)
				}
			})
			if slot != (cjsiDurableSlot{}) || !errors.Is(err, errCJSISlotAdmission) {
				t.Fatalf("accepted altered slot: %+v, %v", slot, err)
			}
			if fault == "staging-name-rebound" {
				checkCJSISlotBytes(t, path, payload)
				if _, err := os.Lstat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("installed a replacement staging inode: %v", err)
				}
			}
		})
	}
}
