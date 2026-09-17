package ingest

import (
	"bufio"
	"errors"
)

// readRecordLine borrows the reader buffer for ordinary records. Callers must
// decode it before the next read and copy any retained incomplete suffix.
// Oversized records retain ReadBytes semantics without imposing a new limit.
func readRecordLine(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadSlice('\n')
	if !errors.Is(err, bufio.ErrBufferFull) {
		return line, err
	}
	complete := append([]byte(nil), line...)
	for errors.Is(err, bufio.ErrBufferFull) {
		line, err = reader.ReadSlice('\n')
		complete = append(complete, line...)
	}
	return complete, err
}
