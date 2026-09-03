package host

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"
)

type osFileSystem struct{}

func (osFileSystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (osFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}

func (osFileSystem) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

func (osFileSystem) RealPath(name string) (string, error) {
	return filepath.EvalSymlinks(name)
}

type systemClock struct {
	origin time.Time
}

func (*systemClock) Now() (time.Time, error) {
	return time.Now(), nil
}

func (clock *systemClock) Monotonic() (time.Duration, error) {
	return time.Since(clock.origin), nil
}

func (*systemClock) Sleep(duration time.Duration) error {
	time.Sleep(duration)
	return nil
}

type osProcess struct{}

func (osProcess) Args() []string {
	return slices.Clone(os.Args)
}

func (osProcess) Environ() []string {
	return os.Environ()
}

func (osProcess) Executable() (string, error) {
	return os.Executable()
}

func (osProcess) Getwd() (string, error) {
	return os.Getwd()
}
