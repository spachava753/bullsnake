package runtime

import (
	"errors"
	"io/fs"
	"testing"
)

func TestProviderErrorVisibleFilename(t *testing.T) {
	visible := "python-visible.txt"
	original := &fs.PathError{Op: "open", Path: "/private/provider/path", Err: fs.ErrNotExist}
	exception := providerException(original, &visible)
	if exception.class != fileNotFoundErrorType {
		t.Fatal("wrong error classification")
	}
	filename, _ := exception.attribute("filename")
	if filename.Repr() != "'python-visible.txt'" {
		t.Fatalf("filename = %v", filename)
	}
	if len(exception.arguments().elements) != 2 {
		t.Fatal("filename leaked into args")
	}
	if exception.Message() != "[Errno 2] file does not exist: 'python-visible.txt'" {
		t.Fatalf("message = %s", exception.Message())
	}
	unknown := providerException(errors.New("provider failure"), nil)
	if unknown.class != osErrorType {
		t.Fatal("unclassified failure lost OSError ancestry")
	}
}
