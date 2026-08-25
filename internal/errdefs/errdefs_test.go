package errdefs_test

import (
	"errors"
	"testing"

	"github.com/kitdine/agent-deck/internal/errdefs"
)

func TestNotFoundRedactsAndPreservesCause(t *testing.T) {
	cause := errors.New("private storage detail")
	err := errdefs.NewNotFound("thing_not_found", "no thing is known", cause)

	if got := err.Error(); got != "no thing is known" {
		t.Fatalf("Error() = %q", got)
	}
	if !errors.Is(err, cause) {
		t.Fatal("NotFound does not preserve its cause")
	}
	var notFound *errdefs.NotFound
	if !errors.As(err, &notFound) || notFound.Code != "thing_not_found" {
		t.Fatalf("errors.As = %#v", notFound)
	}
	withoutCause := &errdefs.NotFound{Code: "thing_not_found", Message: "no thing is known"}
	if errors.Is(withoutCause, cause) {
		t.Fatal("struct literal unexpectedly attached a cause")
	}
}
