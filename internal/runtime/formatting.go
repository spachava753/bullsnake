package runtime

import (
	"fmt"
	"strings"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

func executeConvertValue(
	frame *frame,
	index int,
	conversion uint32,
) (instructionOutcome, error) {
	value, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	var text string
	switch conversion {
	case bytecode.ConversionString:
		text = valueText(value)
	case bytecode.ConversionRepr:
		text = value.Repr()
	case bytecode.ConversionASCII:
		text = asciiRepresentation(value.Repr())
	default:
		return instructionOutcome{}, frame.failure(index, "invalid formatted conversion")
	}
	return pushOutcome(frame, index, &stringValue{value: text})
}

func executeFormatSimple(frame *frame, index int) (instructionOutcome, error) {
	value, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	if _, exactString := value.(*stringValue); exactString {
		return pushOutcome(frame, index, value)
	}
	return pushOutcome(frame, index, &stringValue{value: valueText(value)})
}

// executeFormatWithSpec consumes a value and string specification, preserves
// no-op formatting identities, and reports format failures as Python exceptions.
func executeFormatWithSpec(frame *frame, index int) (instructionOutcome, error) {
	specValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	value, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	spec, ok := specValue.(*stringValue)
	if !ok {
		return instructionOutcome{}, frame.failure(index, "format specification is not a string")
	}
	if spec.value == "" {
		if _, exactString := value.(*stringValue); exactString {
			return pushOutcome(frame, index, value)
		}
		return pushOutcome(frame, index, &stringValue{value: valueText(value)})
	}
	text, ok := value.(*stringValue)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"unsupported format string passed to "+value.TypeName()+".__format__",
			),
		}, nil
	}
	formatted, exception := formatStringValue(text.value, spec.value)
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	if formatted == text.value {
		return pushOutcome(frame, index, text)
	}
	return pushOutcome(frame, index, &stringValue{value: formatted})
}

// executeBuildString joins the top count exact string values in source order.
// Other value types indicate malformed bytecode because formatting produces strings.
func executeBuildString(frame *frame, index, count int) (instructionOutcome, error) {
	start := len(frame.stack) - count
	if start < 0 {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	var builder strings.Builder
	for pieceIndex, piece := range frame.stack[start:] {
		text, ok := piece.(*stringValue)
		if !ok {
			return instructionOutcome{}, frame.failure(
				index,
				fmt.Sprintf("BUILD_STRING item %d is not a string", pieceIndex),
			)
		}
		builder.WriteString(text.value)
	}
	for stackIndex := start; stackIndex < len(frame.stack); stackIndex++ {
		frame.stack[stackIndex] = nil
	}
	frame.stack = frame.stack[:start]
	return pushOutcome(frame, index, &stringValue{value: builder.String()})
}

func valueText(value Value) string {
	switch value := value.(type) {
	case *stringValue:
		return value.value
	case *Exception:
		return value.message
	default:
		return value.Repr()
	}
}

// asciiRepresentation escapes every non-ASCII code point in an existing repr.
func asciiRepresentation(representation string) string {
	var builder strings.Builder
	for _, current := range representation {
		switch {
		case current < 0x80:
			builder.WriteRune(current)
		case current <= 0xff:
			fmt.Fprintf(&builder, `\x%02x`, current)
		case current <= 0xffff:
			fmt.Fprintf(&builder, `\u%04x`, current)
		default:
			fmt.Fprintf(&builder, `\U%08x`, current)
		}
	}
	return builder.String()
}

type stringFormatSpec struct {
	fill      string
	alignment byte
	width     int
	precision int
}

// formatStringValue parses and applies the format mini-language supported by str.
func formatStringValue(text, specification string) (string, *Exception) {
	spec, exception := parseStringFormatSpec(specification)
	if exception != nil {
		return "", exception
	}
	offsets := stringCodepointOffsets(text)
	length := len(offsets) - 1
	if spec.precision >= 0 && spec.precision < length {
		text = text[:offsets[spec.precision]]
		length = spec.precision
	}
	padding := spec.width - length
	if padding <= 0 {
		return text, nil
	}
	maxInt := int(^uint(0) >> 1)
	if len(spec.fill) != 0 && padding > maxInt/len(spec.fill) {
		return "", newException("ValueError", "format specifier is too large")
	}
	left, right := 0, 0
	switch spec.alignment {
	case '<':
		right = padding
	case '>':
		left = padding
	case '^':
		left = padding / 2
		right = padding - left
	}
	return strings.Repeat(spec.fill, left) + text + strings.Repeat(spec.fill, right), nil
}

// parseStringFormatSpec accepts one fill code point, string alignment, width,
// precision, and the optional s presentation type.
func parseStringFormatSpec(text string) (stringFormatSpec, *Exception) {
	spec := stringFormatSpec{fill: " ", alignment: '<', width: -1, precision: -1}
	position := 0
	if text != "" {
		_, firstSize, _ := decodeStringRune(text)
		if firstSize < len(text) && isFormatAlignment(text[firstSize]) {
			spec.fill = text[:firstSize]
			spec.alignment = text[firstSize]
			position = firstSize + 1
		} else if firstSize == 1 && isFormatAlignment(text[0]) {
			spec.alignment = text[0]
			position = 1
		}
	}
	if spec.alignment == '=' {
		return stringFormatSpec{}, newException(
			"ValueError",
			"'=' alignment not allowed in string format specifier",
		)
	}
	if position < len(text) && strings.ContainsRune("+- ", rune(text[position])) {
		return stringFormatSpec{}, newException(
			"ValueError",
			"Sign not allowed in string format specifier",
		)
	}
	if position < len(text) && text[position] == '0' {
		return stringFormatSpec{}, newException(
			"ValueError",
			"'=' alignment not allowed in string format specifier",
		)
	}
	var exception *Exception
	spec.width, position, exception = parseFormatNumber(text, position)
	if exception != nil {
		return stringFormatSpec{}, exception
	}
	if position < len(text) && (text[position] == ',' || text[position] == '_') {
		return stringFormatSpec{}, invalidStringFormat(text)
	}
	if position < len(text) && text[position] == '.' {
		position++
		spec.precision, position, exception = parseFormatNumber(text, position)
		if exception != nil {
			return stringFormatSpec{}, exception
		}
		if spec.precision < 0 {
			return stringFormatSpec{}, newException(
				"ValueError",
				"Format specifier missing precision",
			)
		}
	}
	remaining := text[position:]
	if remaining == "" || remaining == "s" {
		return spec, nil
	}
	_, size, _ := decodeStringRune(remaining)
	if size == len(remaining) {
		return stringFormatSpec{}, newException(
			"ValueError",
			"Unknown format code '"+remaining+"' for object of type 'str'",
		)
	}
	return stringFormatSpec{}, invalidStringFormat(text)
}

// parseFormatNumber consumes decimal digits without overflowing a Go int and
// leaves the cursor unchanged when the field is absent.
func parseFormatNumber(text string, position int) (int, int, *Exception) {
	start := position
	value := 0
	maxInt := int(^uint(0) >> 1)
	for position < len(text) && text[position] >= '0' && text[position] <= '9' {
		digit := int(text[position] - '0')
		if value > (maxInt-digit)/10 {
			return 0, position, newException(
				"ValueError",
				"Too many decimal digits in format string",
			)
		}
		value = value*10 + digit
		position++
	}
	if position == start {
		return -1, position, nil
	}
	return value, position, nil
}

func isFormatAlignment(value byte) bool {
	return value == '<' || value == '>' || value == '=' || value == '^'
}

func invalidStringFormat(text string) *Exception {
	return newException(
		"ValueError",
		"Invalid format specifier "+quoteString(text)+" for object of type 'str'",
	)
}
