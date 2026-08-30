package bytecode

import (
	"strconv"
	"strings"
)

// Dump returns a stable structural representation of a code object.
func Dump(code *Code) string {
	if code == nil {
		return "nil"
	}
	var builder strings.Builder
	builder.WriteString("Code(name=")
	builder.WriteString(strconv.Quote(code.name))
	builder.WriteString(", qualname=")
	builder.WriteString(strconv.Quote(code.qualifiedName))
	builder.WriteString(", filename=")
	builder.WriteString(strconv.Quote(code.filename))
	builder.WriteString(", first_line=")
	builder.WriteString(strconv.Itoa(code.firstLine))
	builder.WriteString(", flags=[")
	builder.WriteString(code.flags.String())
	builder.WriteByte(']')
	if code.name != "<module>" {
		builder.WriteString(", args=(posonly=")
		builder.WriteString(strconv.Itoa(code.positionalOnlyCount))
		builder.WriteString(", positional=")
		builder.WriteString(strconv.Itoa(code.positionalCount))
		builder.WriteString(", kwonly=")
		builder.WriteString(strconv.Itoa(code.keywordOnlyCount))
		builder.WriteByte(')')
	}
	builder.WriteString(", stack=")
	builder.WriteString(strconv.Itoa(code.stackSize))
	builder.WriteString(", constants=[")
	for index, constant := range code.constants {
		if index != 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(constant.String())
	}
	builder.WriteString("], names=")
	dumpStrings(&builder, code.names)
	builder.WriteString(", locals=")
	dumpStrings(&builder, code.locals)
	builder.WriteString(", cells=")
	dumpStrings(&builder, code.cells)
	builder.WriteString(", free=")
	dumpStrings(&builder, code.freeVars)
	if len(code.exceptionHandlers) != 0 {
		builder.WriteString(", handlers=[")
		for index, handler := range code.exceptionHandlers {
			if index != 0 {
				builder.WriteString(", ")
			}
			builder.WriteString("Handler(start=")
			builder.WriteString(strconv.FormatUint(uint64(handler.Start), 10))
			builder.WriteString(", end=")
			builder.WriteString(strconv.FormatUint(uint64(handler.End), 10))
			builder.WriteString(", target=")
			builder.WriteString(strconv.FormatUint(uint64(handler.Target), 10))
			builder.WriteString(", depth=")
			builder.WriteString(strconv.Itoa(handler.StackDepth))
			builder.WriteByte(')')
		}
		builder.WriteByte(']')
	}
	if len(code.children) != 0 {
		builder.WriteString(", children=[")
		for index, child := range code.children {
			if index != 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(Dump(child))
		}
		builder.WriteByte(']')
	}
	builder.WriteString(", instructions=[")
	for index, instruction := range code.instructions {
		if index != 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(instruction.String())
	}
	builder.WriteString("])")
	return builder.String()
}

func dumpStrings(builder *strings.Builder, values []string) {
	builder.WriteByte('[')
	for index, value := range values {
		if index != 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(strconv.Quote(value))
	}
	builder.WriteByte(']')
}
