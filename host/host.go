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
// the first os module subset. Mutation is deliberately kept in FileMutator so
// existing read-only implementations remain valid.
type FileSystem interface {
	ReadFile(name string) ([]byte, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	Stat(name string) (fs.FileInfo, error)
	RealPath(name string) (string, error)
}

// FileMutator is the filesystem mutation surface used by operations such as
// os.remove and os.unlink. It is separate from FileSystem so an embedder can
// grant read access without implicitly granting write access.
type FileMutator interface {
	Remove(name string) error
	RemoveDir(name string) error
}

// Clock supplies wall, monotonic, and sleeping operations for the time module.
// Errors allow a deterministic or policy-enforcing clock to deny an operation.
type Clock interface {
	Now() (time.Time, error)
	Monotonic() (time.Duration, error)
	Sleep(time.Duration) error
}

// TimeZone converts an instant through host-local civil-time rules. Keeping it
// separate lets deterministic embedders replace or deny ambient zone data.
type TimeZone interface {
	LocalTime(time.Time) (time.Time, error)
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

// WorkingDirectory controls mutation of the process working directory.
// It is separate from Process so read-only process metadata can be granted
// without allowing interpreter code to change host-global state.
type WorkingDirectory interface {
	Chdir(string) error
}

// Entropy supplies cryptographically secure random bytes for operations such
// as os.urandom. It is separate from the filesystem and process capabilities
// so deterministic tests and policy engines can replace or deny it directly.
type Entropy interface {
	Read([]byte) (int, error)
}

// Services groups independently replaceable host capabilities. Nil fields deny
// that capability; callers can therefore grant only what an interpreter needs.
type Services struct {
	Files            FileSystem
	FileMutator      FileMutator
	Clock            Clock
	TimeZone         TimeZone
	Network          Network
	Process          Process
	WorkingDirectory WorkingDirectory
	Entropy          Entropy
	Stdin            io.Reader
	Stdout           io.Writer
	Stderr           io.Writer
}

// Default returns capabilities backed by the current process and operating
// system. Embedders should replace or clear fields before creating a runtime.
func Default() Services {
	return Services{
		Files:            osFileSystem{},
		FileMutator:      osFileMutator{},
		Clock:            &systemClock{origin: time.Now()},
		TimeZone:         systemTimeZone{},
		Network:          &net.Dialer{},
		Process:          osProcess{},
		WorkingDirectory: osProcess{},
		Entropy:          systemEntropy{},
		Stdin:            os.Stdin,
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
	}
}
