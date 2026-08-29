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
	falseSingleton    Value = &boolValue{}
	trueSingleton     Value = &boolValue{value: true}
	ellipsisSingleton Value = &ellipsisValue{}
)

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

type bytesValue struct {
	value string
}

func (*bytesValue) TypeName() string   { return "bytes" }
func (value *bytesValue) Repr() string { return quoteBytes(value.value) }
func (*bytesValue) isValue()           {}

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
