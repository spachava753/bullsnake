package ast

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// DumpOptions controls the stable textual AST representation used by tests and diagnostics.
type DumpOptions struct {
	IncludeSpans bool
}

// Dump returns a deterministic representation of an AST.
func Dump(root Node, options DumpOptions) string {
	var builder strings.Builder
	dumpValue(&builder, reflect.ValueOf(root), options)
	return builder.String()
}

// dumpValue formats AST values recursively, honoring named enum strings and node spans.
func dumpValue(builder *strings.Builder, value reflect.Value, options DumpOptions) {
	for value.IsValid() && value.Kind() == reflect.Interface {
		if value.IsNil() {
			builder.WriteString("nil")
			return
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		builder.WriteString("nil")
		return
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			builder.WriteString("nil")
			return
		}
		if node, ok := value.Interface().(Node); ok {
			dumpStruct(builder, value.Elem(), node, options)
			return
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Struct:
		dumpStruct(builder, value, nil, options)
	case reflect.Slice, reflect.Array:
		builder.WriteByte('[')
		for index := range value.Len() {
			if index != 0 {
				builder.WriteString(", ")
			}
			dumpValue(builder, value.Index(index), options)
		}
		builder.WriteByte(']')
	case reflect.String:
		builder.WriteString(strconv.Quote(value.String()))
	case reflect.Bool:
		builder.WriteString(strconv.FormatBool(value.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value.CanInterface() {
			if stringer, ok := value.Interface().(fmt.Stringer); ok {
				builder.WriteString(stringer.String())
				return
			}
		}
		builder.WriteString(strconv.FormatInt(value.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if value.CanInterface() {
			if stringer, ok := value.Interface().(fmt.Stringer); ok {
				builder.WriteString(stringer.String())
				return
			}
		}
		builder.WriteString(strconv.FormatUint(value.Uint(), 10))
	default:
		panic(fmt.Sprintf("ast.Dump: unsupported %s value", value.Kind()))
	}
}

// dumpStruct emits exported AST fields in declaration order and appends an optional node span.
func dumpStruct(builder *strings.Builder, value reflect.Value, node Node, options DumpOptions) {
	builder.WriteString(value.Type().Name())
	builder.WriteByte('(')
	wroteField := false
	for index := range value.NumField() {
		fieldType := value.Type().Field(index)
		if fieldType.PkgPath != "" || fieldType.Name == "Range" {
			continue
		}
		if wroteField {
			builder.WriteString(", ")
		}
		wroteField = true
		fieldName := strings.ToLower(fieldType.Name[:1]) + fieldType.Name[1:]
		if fieldType.Name == "ID" {
			fieldName = "id"
		}
		builder.WriteString(fieldName)
		builder.WriteByte('=')
		dumpValue(builder, value.Field(index), options)
	}
	builder.WriteByte(')')
	if node != nil && options.IncludeSpans {
		span := node.Span()
		fmt.Fprintf(
			builder,
			"@%d:%d-%d:%d",
			span.Start.Line,
			span.Start.Column,
			span.End.Line,
			span.End.Column,
		)
	}
}
