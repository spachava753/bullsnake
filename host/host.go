// Package host defines the capabilities through which Bullsnake reaches its host.
package host

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"time"
)

// ErrDenied lets a capability wrapper reject an operation without exposing a
// platform-specific permission error.
var ErrDenied = errors.New("host operation denied")

// FileSystem is the read-only filesystem surface used by source imports and
// the first os module subset. Future mutation support belongs in a separate
// interface so existing read-only implementations remain valid.
type FileSystem interface {
	ReadFile(name string) ([]byte, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	Stat(name string) (fs.FileInfo, error)
	RealPath(name string) (string, error)
}

// Clock supplies wall, monotonic, and sleeping operations for the time module.
// Errors allow a deterministic or policy-enforcing clock to deny an operation.
type Clock interface {
	Now() (time.Time, error)
	Monotonic() (time.Duration, error)
	Sleep(time.Duration) error
}

// Network is the initial outbound network capability. Socket modules must use
// this boundary rather than constructing a net.Dialer directly.
type Network interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

// Process supplies process data without making os package globals ambient
// interpreter authority.
type Process interface {
	Args() []string
	Environ() []string
	Executable() (string, error)
	Getwd() (string, error)
}

// Services groups independently replaceable host capabilities. Nil fields deny
// that capability; callers can therefore grant only what an interpreter needs.
type Services struct {
	Files   FileSystem
	Clock   Clock
	Network Network
	Process Process
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

// Default returns capabilities backed by the current process and operating
// system. Embedders should replace or clear fields before creating a runtime.
func Default() Services {
	return Services{
		Files:   osFileSystem{},
		Clock:   &systemClock{origin: time.Now()},
		Network: &net.Dialer{},
		Process: osProcess{},
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}
}
