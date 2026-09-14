package ingest

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReadRecordLinePreservesBoundariesAndOversizedTail(t *testing.T) {
	for _, input := range []string{"", "one\nlast", strings.Repeat("x", 64) + "\nnext\n", strings.Repeat("x", 8192) + "\n" + strings.Repeat("y", 8192)} {
		reference := bufio.NewReaderSize(strings.NewReader(input), 64)
		borrowed := bufio.NewReaderSize(strings.NewReader(input), 64)
		for {
			want, wantErr := reference.ReadBytes('\n')
			got, err := readRecordLine(borrowed)
			if string(got) != string(want) || !errors.Is(err, wantErr) {
				t.Fatalf("record differs: got=%d/%v want=%d/%v", len(got), err, len(want), wantErr)
			}
			if errors.Is(err, io.EOF) {
				break
			}
		}
	}
}

func BenchmarkRecordLine(b *testing.B) {
	input := strings.Repeat(strings.Repeat("x", 1023)+"\n", 1024)
	for _, borrow := range []bool{false, true} {
		name := "copy"
		if borrow {
			name = "borrow"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			reader := bufio.NewReaderSize(strings.NewReader(input), 1<<20)
			for i := 0; i < b.N; i++ {
				reader.Reset(strings.NewReader(input))
				for {
					var err error
					if borrow {
						_, err = readRecordLine(reader)
					} else {
						_, err = reader.ReadBytes('\n')
					}
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
