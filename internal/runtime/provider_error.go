package runtime

import (
	"errors"
	"io"
	"io/fs"
	"syscall"
)

// providerException classifies wrapped Go errors and strips provider-only paths.
// A Python filename is included only when the adapter explicitly supplies it.
func providerException(err error, filename *string) *Exception {
	number := providerErrno(err)
	var pathError *fs.PathError
	if errors.As(err, &pathError) {
		err = pathError.Err
	}
	args := []Value{integerFromInt64(int64(number)), &stringValue{value: err.Error()}}
	if filename != nil {
		args = append(args, &stringValue{value: *filename})
	}
	exception := newExceptionOfType(osErrorType, "")
	exception.setArguments(args)
	return exception
}

// providerErrno translates recognized host errors to the fixed Python errno
// vocabulary without leaking host-specific numeric errno assignments.
func providerErrno(err error) int {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return 13
	case errors.Is(err, fs.ErrNotExist):
		return 2
	case errors.Is(err, fs.ErrClosed):
		return 9
	case errors.Is(err, fs.ErrInvalid):
		return 22
	case errors.Is(err, fs.ErrExist):
		return 17
	case errors.Is(err, io.ErrClosedPipe), errors.Is(err, syscall.EPIPE):
		return 32
	case errors.Is(err, syscall.ENOTDIR):
		return 20
	case errors.Is(err, syscall.EISDIR):
		return 21
	case errors.Is(err, syscall.EAGAIN):
		return 11
	case errors.Is(err, syscall.EINTR):
		return 4
	case errors.Is(err, syscall.ETIMEDOUT):
		return 110
	case errors.Is(err, syscall.ECONNRESET):
		return 104
	case errors.Is(err, syscall.ECONNREFUSED):
		return 111
	case errors.Is(err, syscall.ECONNABORTED):
		return 103
	default:
		return 5
	}
}
