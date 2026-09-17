package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SQLite's read-only WAL connections can still create or update shared memory.
// Recover a private copy instead, including committed WAL frames. Refuse a
// changing source rather than diagnosing a version from a torn copy.
func schemaProbeSnapshot(ctx context.Context, root string) (string, func(), error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp("", "agentdeck-schema-probe-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	ok := false
	defer func() {
		if !ok {
			cleanup()
		}
	}()
	name := "agentdeck.sqlite3"
	type sourceFile struct {
		suffix string
		exists bool
		digest [32]byte
		info   os.FileInfo
	}
	files := []sourceFile{{suffix: ""}, {suffix: "-wal"}, {suffix: "-journal"}}
	for i := range files {
		file := &files[i]
		source, err := os.Open(filepath.Join(root, name+file.suffix))
		if os.IsNotExist(err) && file.suffix != "" {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		file.exists = true
		file.info, err = source.Stat()
		if err != nil || !file.info.Mode().IsRegular() {
			source.Close()
			return "", nil, fmt.Errorf("invalid probe source")
		}
		// A rollback journal may represent an unfinished transaction. Do not
		// recover it without the state lock or mistake its pages for committed data.
		if file.suffix == "-journal" && file.info.Size() != 0 {
			source.Close()
			return "", nil, fmt.Errorf("active rollback journal")
		}
		dest, err := os.OpenFile(filepath.Join(dir, name+file.suffix), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			source.Close()
			return "", nil, err
		}
		hash := sha256.New()
		_, err = io.Copy(io.MultiWriter(dest, hash), probeContextReader{ctx, source})
		source.Close()
		closeErr := dest.Close()
		if err != nil {
			return "", nil, err
		}
		if closeErr != nil {
			return "", nil, closeErr
		}
		copy(file.digest[:], hash.Sum(nil))
	}
	for _, file := range files {
		source, err := os.Open(filepath.Join(root, name+file.suffix))
		if !file.exists && os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		info, statErr := source.Stat()
		if !file.exists || statErr != nil || !os.SameFile(file.info, info) || file.info.Size() != info.Size() || !file.info.ModTime().Equal(info.ModTime()) {
			source.Close()
			return "", nil, fmt.Errorf("probe source changed")
		}
		hash := sha256.New()
		_, err = io.Copy(hash, probeContextReader{ctx, source})
		source.Close()
		if err != nil {
			return "", nil, err
		}
		var digest [32]byte
		copy(digest[:], hash.Sum(nil))
		if digest != file.digest {
			return "", nil, fmt.Errorf("probe source changed")
		}
	}
	ok = true
	return filepath.Join(dir, name), cleanup, nil
}

type probeContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r probeContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
