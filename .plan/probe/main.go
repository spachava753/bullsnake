package main

import (
 "fmt"
 "os"
 "github.com/spachava753/bullsnake/internal/importer"
 rt "github.com/spachava753/bullsnake/internal/runtime"
)

func main() {
 loader := importer.NewFileSystem(os.Args[1])
 names := os.Args[2:]
 if len(names) == 0 { names = []string{"operator", "keyword", "heapq", "abc", "unittest"} }
 for _, name := range names {
  spec, found, err := loader.Load(rt.ModuleRequest{Name:name})
  if err == nil && found { _, err = rt.NewWithLoader(loader.Load).ExecuteModuleSpec(name, spec) }
  fmt.Printf("%s: found=%t %v\n", name, found, err)
 }
}
