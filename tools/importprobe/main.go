package main

import (
	"fmt"
	"os"

	"github.com/spachava753/bullsnake/internal/importer"
	rt "github.com/spachava753/bullsnake/internal/runtime"
)

// main probes each requested import in a fresh runtime and reports failures.
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: importprobe SOURCE_ROOT [MODULE ...]")
		os.Exit(2)
	}
	failed := false
	loader := importer.NewFileSystem(os.Args[1])
	names := os.Args[2:]
	if len(names) == 0 {
		names = []string{"operator", "keyword", "heapq", "abc", "unittest"}
	}
	for _, name := range names {
		spec, found, err := loader.Load(rt.ModuleRequest{Name: name})
		if err == nil && found {
			_, err = rt.NewWithLoader(loader.Load).ExecuteModuleSpec(name, spec)
		}
		fmt.Printf("%s: found=%t %v\n", name, found, err)
		if !found || err != nil {
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}
