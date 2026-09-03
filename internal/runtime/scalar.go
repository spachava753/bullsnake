package runtime

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type boolValue struct {
	value bool
}

var (
	falseSingleton          Value = &boolValue{}
	trueSingleton           Value = &boolValue{value: true}
	ellipsisSingleton       Value = &ellipsisValue{}
	notImplementedSingleton Value = &notImplementedValue{marker: 1}
)

type notImplementedValue struct{ marker byte }

func (*notImplementedValue) TypeName() string { return "NotImplementedType" }
func (*notImplementedValue) Repr() string     { return "NotImplemented" }
func (*notImplementedValue) isValue()         {}

func (*boolValue) TypeName() string { return "bool" }
func (value *boolValue) Repr() string {
	if value.value {
		return "True"
	}
	return "False"
}
func (*boolValue) isValue() {}

type ellipsisValue struct{}

func (*ellipsisValue) TypeName() string { return "ellipsis" }
func (*ellipsisValue) Repr() string     { return "Ellipsis" }
func (*ellipsisValue) isValue()         {}

type floatValue struct {
	value float64
}

func (*floatValue) TypeName() string   { return "float" }
func (value *floatValue) Repr() string { return formatFloat(value.value, true) }
func (*floatValue) isValue()           {}

type complexValue struct {
	real      float64
	imaginary float64
}

func (*complexValue) TypeName() string { return "complex" }
func (value *complexValue) Repr() string {
	if value.real == 0 && !math.Signbit(value.real) {
		return formatFloat(value.imaginary, false) + "j"
	}
	sign := "+"
	imaginary := value.imaginary
	if math.Signbit(imaginary) {
		sign = "-"
		imaginary = -imaginary
	}
	return "(" + formatFloat(value.real, false) + sign +
		formatFloat(imaginary, false) + "j)"
}
func (*complexValue) isValue() {}

type stringValue struct {
	value string
}

func (*stringValue) TypeName() string   { return "str" }
func (value *stringValue) Repr() string { return quoteString(value.value) }
func (*stringValue) isValue()           {}

func (value *stringValue) attribute(name string) (Value, bool) {
	return stringMethod(value, name)
}

type bytesValue struct {
	value string
}

func (*bytesValue) TypeName() string   { return "bytes" }
func (value *bytesValue) Repr() string { return quoteBytes(value.value) }
func (*bytesValue) isValue()           {}

// attribute exposes byte decoding and prefix matching needed by source
// encoding detection while preserving byte-oriented arguments and results.
func (value *bytesValue) attribute(name string) (Value, bool) {
	switch name {
	case "decode":
		return nativeFunctionNamed("bytes.decode", 0, 2,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return &stringValue{value: value.value}, nil, nil
			}), true
	case "startswith", "endswith":
		return nativeFunctionNamed("bytes."+name, 1, 3,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				selected := value.value
				if len(arguments) >= 2 {
					start, ok := integerOperand(arguments[1])
					if !ok || !start.IsInt64() {
						return nil, newException("TypeError", "slice indices must be integers"), nil
					}
					position := max(int64(0), min(start.Int64(), int64(len(selected))))
					selected = selected[position:]
				}
				if len(arguments) == 3 {
					end, ok := integerOperand(arguments[2])
					if !ok || !end.IsInt64() {
						return nil, newException("TypeError", "slice indices must be integers"), nil
					}
					position := max(int64(0), min(end.Int64(), int64(len(selected))))
					selected = selected[:position]
				}
				candidates := []Value{arguments[0]}
				if tuple, ok := arguments[0].(*tupleValue); ok {
					candidates = tuple.elements
				}
				for _, choice := range candidates {
					var candidate string
					switch prefix := choice.(type) {
					case *bytesValue:
						candidate = prefix.value
					case *bytearrayValue:
						candidate = prefix.value
					default:
						return nil, newException("TypeError", name+" first arg must be bytes-like"), nil
					}
					matched := strings.HasPrefix(selected, candidate)
					if name == "endswith" {
						matched = strings.HasSuffix(selected, candidate)
					}
					if matched {
						return trueSingleton, nil, nil
					}
				}
				return falseSingleton, nil, nil
			}), true
	case "splitlines":
		return nativeFunctionNamed("bytes.splitlines", 0, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				keepEnds := len(arguments) == 1 && truthValue(arguments[0])
				parts := splitByteLines(value.value, keepEnds)
				elements := make([]Value, len(parts))
				for index, part := range parts {
					elements[index] = &bytesValue{value: part}
				}
				return &listValue{elements: elements}, nil, nil
			}), true
	case "count":
		return nativeFunctionNamed("bytes.count", 1, 3,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				needle, ok := arguments[0].(*bytesValue)
				if !ok {
					return nil, newException("TypeError", "argument should be bytes-like"), nil
				}
				return newInt64(int64(strings.Count(value.value, needle.value))), nil, nil
			}), true
	default:
		return nil, false
	}
}

// splitByteLines separates CR, LF, and CRLF byte records with optional endings.
func splitByteLines(value string, keepEnds bool) []string {
	if value == "" {
		return nil
	}
	parts := make([]string, 0)
	start := 0
	for index := 0; index < len(value); index++ {
		if value[index] != '\n' && value[index] != '\r' {
			continue
		}
		end := index
		if value[index] == '\r' && index+1 < len(value) && value[index+1] == '\n' {
			index++
		}
		if keepEnds {
			end = index + 1
		}
		parts = append(parts, value[start:end])
		start = index + 1
	}
	if start < len(value) {
		parts = append(parts, value[start:])
	}
	return parts
}

// formatFloat renders finite values with Go's shortest round-trip spelling and
// optionally retains the decimal marker required by a Python float repr.
func formatFloat(value float64, forceDecimal bool) string {
	switch {
	case math.IsNaN(value):
		return "nan"
	case math.IsInf(value, 1):
		return "inf"
	case math.IsInf(value, -1):
		return "-inf"
	}
	formatted := strconv.FormatFloat(value, 'g', -1, 64)
	if forceDecimal && !strings.ContainsAny(formatted, ".e") {
		formatted += ".0"
	}
	return formatted
}

func validStringEncoding(value string) bool {
	for index := 0; index < len(value); {
		_, size, ok := decodeStringRune(value[index:])
		if !ok {
			return false
		}
		index += size
	}
	return true
}

func quoteString(value string) string {
	quote := byte('\'')
	if strings.ContainsRune(value, '\'') && !strings.ContainsRune(value, '"') {
		quote = '"'
	}
	var builder strings.Builder
	builder.WriteByte(quote)
	for index := 0; index < len(value); {
		current, size, _ := decodeStringRune(value[index:])
		writeQuotedRune(&builder, current, quote)
		index += size
	}
	builder.WriteByte(quote)
	return builder.String()
}

// decodeStringRune accepts ordinary UTF-8 and the three-byte WTF-8 sequences
// that preserve lone surrogate escapes in compiler string constants.
func decodeStringRune(value string) (current rune, size int, ok bool) {
	if len(value) >= 3 && value[0] == 0xed && value[1] >= 0xa0 &&
		value[1] <= 0xbf && value[2] >= 0x80 && value[2] <= 0xbf {
		current = rune(value[0]&0x0f)<<12 |
			rune(value[1]&0x3f)<<6 |
			rune(value[2]&0x3f)
		return current, 3, true
	}
	current, size = utf8.DecodeRuneInString(value)
	if current == utf8.RuneError && size == 1 {
		return 0, 0, false
	}
	return current, size, true
}

// writeQuotedRune applies Python-style escapes while retaining printable
// Unicode and emitting lone surrogates as explicit escape sequences.
func writeQuotedRune(builder *strings.Builder, current rune, quote byte) {
	switch current {
	case '\\':
		builder.WriteString(`\\`)
	case '\a':
		builder.WriteString(`\a`)
	case '\b':
		builder.WriteString(`\b`)
	case '\f':
		builder.WriteString(`\f`)
	case '\n':
		builder.WriteString(`\n`)
	case '\r':
		builder.WriteString(`\r`)
	case '\t':
		builder.WriteString(`\t`)
	case '\v':
		builder.WriteString(`\v`)
	case rune(quote):
		builder.WriteByte('\\')
		builder.WriteByte(quote)
	default:
		switch {
		case current >= 0xd800 && current <= 0xdfff:
			builder.WriteString(fmt.Sprintf(`\u%04x`, current))
		case unicode.IsPrint(current):
			builder.WriteRune(current)
		case current <= 0xff:
			builder.WriteString(fmt.Sprintf(`\x%02x`, current))
		case current <= 0xffff:
			builder.WriteString(fmt.Sprintf(`\u%04x`, current))
		default:
			builder.WriteString(fmt.Sprintf(`\U%08x`, current))
		}
	}
}

// quoteBytes renders printable ASCII directly and all other payload bytes with
// the escape forms used by Python bytes representations.
func quoteBytes(value string) string {
	quote := byte('\'')
	if strings.ContainsRune(value, '\'') && !strings.ContainsRune(value, '"') {
		quote = '"'
	}
	var builder strings.Builder
	builder.WriteByte('b')
	builder.WriteByte(quote)
	for index := 0; index < len(value); index++ {
		current := value[index]
		switch current {
		case '\\':
			builder.WriteString(`\\`)
		case '\t':
			builder.WriteString(`\t`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case quote:
			builder.WriteByte('\\')
			builder.WriteByte(quote)
		default:
			if current >= 0x20 && current <= 0x7e {
				builder.WriteByte(current)
			} else {
				builder.WriteString(fmt.Sprintf(`\x%02x`, current))
			}
		}
	}
	builder.WriteByte(quote)
	return builder.String()
}
