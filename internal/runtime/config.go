package runtime

import (
	"fmt"
	"reflect"
	"slices"
	"time"
	"unicode/utf8"
)

// Config supplies isolated interpreter data and source loading. No omitted
// capability acquires ambient process access. Arguments must contain UTF-8.
type Config struct {
	Args    []string
	Loader  ModuleLoader
	Counter PerfCounter
}

// NewWithConfig copies argument data and retains the supplied source loader.
// Empty arguments initialize sys.argv to [""].
func NewWithConfig(config Config) (*Runtime, error) {
	for index, arg := range config.Args {
		if !utf8.ValidString(arg) {
			return nil, fmt.Errorf("argument %d is not valid UTF-8", index)
		}
	}
	if isNilProvider(config.Counter) {
		return nil, fmt.Errorf("counter is a typed nil provider")
	}
	runtime := newRuntime(config.Loader)
	if len(config.Args) != 0 {
		runtime.args = slices.Clone(config.Args)
	}
	runtime.counter = config.Counter
	return runtime, nil
}

// PerfCounter returns nondecreasing elapsed time from a fixed arbitrary origin.
// It grants no wall-clock, sleeping, or scheduling authority.
type PerfCounter interface {
	PerfCounter() time.Duration
}

// isNilProvider rejects typed nil capabilities before Python can call them.
func isNilProvider(provider any) bool {
	if provider == nil {
		return false
	}
	value := reflect.ValueOf(provider)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
