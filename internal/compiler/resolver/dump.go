package resolver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Dump returns a stable, readable representation of a resolver table.
func Dump(table *Table) string {
	if table == nil {
		return "nil"
	}
	var builder strings.Builder
	builder.WriteString("Table(features=")
	dumpFeatures(&builder, table.Features)
	builder.WriteString(", root=")
	dumpScope(&builder, table.Root)
	builder.WriteByte(')')
	return builder.String()
}

func dumpFeatures(builder *strings.Builder, features Features) {
	builder.WriteByte('[')
	wrote := false
	if features&FutureAnnotations != 0 {
		builder.WriteString("FutureAnnotations")
		wrote = true
	}
	if remaining := features &^ FutureAnnotations; remaining != 0 {
		if wrote {
			builder.WriteString(", ")
		}
		fmt.Fprintf(builder, "Features(%#x)", uint8(remaining))
	}
	builder.WriteByte(']')
}

// dumpScope writes one scope and recursively includes children in source order.
func dumpScope(builder *strings.Builder, scope *Scope) {
	if scope == nil {
		builder.WriteString("nil")
		return
	}
	builder.WriteString(scope.Kind.String())
	builder.WriteString("(name=")
	builder.WriteString(strconv.Quote(scope.Name))
	builder.WriteString(", private=")
	builder.WriteString(strconv.Quote(scope.PrivateName))
	builder.WriteString(", flags=")
	dumpFlags(builder, scope.Flags.String())
	builder.WriteString(", parameters=[")
	for index, parameter := range scope.Parameters {
		if index != 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(strconv.Quote(parameter))
	}
	builder.WriteString("], symbols=[")
	dumpSymbols(builder, scope)
	builder.WriteString("], children=[")
	for index, child := range scope.Children {
		if index != 0 {
			builder.WriteString(", ")
		}
		dumpScope(builder, child)
	}
	builder.WriteString("])")
}

// dumpSymbols writes recorded source order first and sorts synthesized names.
func dumpSymbols(builder *strings.Builder, scope *Scope) {
	seen := make(map[string]struct{}, len(scope.Symbols))
	wrote := false
	writeSymbol := func(name string) {
		if _, duplicate := seen[name]; duplicate {
			return
		}
		seen[name] = struct{}{}
		if wrote {
			builder.WriteString(", ")
		}
		wrote = true
		builder.WriteString(name)
		builder.WriteByte('=')
		symbol := scope.Symbols[name]
		if symbol == nil {
			builder.WriteString("nil")
			return
		}
		builder.WriteString(symbol.Resolution.String())
		dumpFlags(builder, symbol.Flags.String())
	}
	for _, name := range scope.SymbolOrder {
		writeSymbol(name)
	}
	var remaining []string
	for name := range scope.Symbols {
		if _, ok := seen[name]; !ok {
			remaining = append(remaining, name)
		}
	}
	sort.Strings(remaining)
	for _, name := range remaining {
		writeSymbol(name)
	}
}

func dumpFlags(builder *strings.Builder, flags string) {
	builder.WriteByte('[')
	if flags != "" {
		builder.WriteString(strings.ReplaceAll(flags, "|", ", "))
	}
	builder.WriteByte(']')
}
