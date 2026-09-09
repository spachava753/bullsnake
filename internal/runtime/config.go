package runtime

import (
	"fmt"
	"slices"
	"unicode/utf8"
)

// Config supplies isolated interpreter data and source loading. No omitted
// capability acquires ambient process access. Arguments must contain UTF-8.
type Config struct {
	Args   []string
	Loader ModuleLoader
}

// NewWithConfig copies argument data and retains the supplied source loader.
// Empty arguments initialize sys.argv to [""].
func NewWithConfig(config Config) (*Runtime, error) {
	for index, arg := range config.Args {
		if !utf8.ValidString(arg) {
			return nil, fmt.Errorf("argument %d is not valid UTF-8", index)
		}
	}
	runtime := newRuntime(config.Loader)
	if len(config.Args) != 0 {
		runtime.args = slices.Clone(config.Args)
	}
	return runtime, nil
}
