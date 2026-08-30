package runtime

import (
	"fmt"
	"math/big"
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
	var formatted string
	var exception *Exception
	original, exactString := value.(*stringValue)
	switch value := value.(type) {
	case *stringValue:
		formatted, exception = formatStringValue(value.value, spec.value)
	case *intValue:
		formatted, exception = formatIntegerValue(&value.value, spec.value, "int")
	case *boolValue:
		var integer big.Int
		if value.value {
			integer.SetInt64(1)
		}
		formatted, exception = formatIntegerValue(&integer, spec.value, "bool")
	default:
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"unsupported format string passed to "+value.TypeName()+".__format__",
			),
		}, nil
	}
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	if exactString && formatted == original.value {
		return pushOutcome(frame, index, original)
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

type integerFormatSpec struct {
	fill              string
	alignment         byte
	explicitAlignment bool
	sign              byte
	alternate         bool
	zero              bool
	width             int
	grouping          byte
	typeCode          byte
}

// formatIntegerValue renders one arbitrary-precision integer according to the
// supported integer presentation types and preserves sign/prefix alignment.
func formatIntegerValue(value *big.Int, specification, typeName string) (string, *Exception) {
	spec, exception := parseIntegerFormatSpec(specification, typeName)
	if exception != nil {
		return "", exception
	}
	typeCode := spec.typeCode
	if typeCode == 0 {
		typeCode = 'd'
	}
	if typeCode == 'c' {
		return formatIntegerCharacter(value, spec)
	}
	base := 10
	prefix := ""
	switch typeCode {
	case 'b':
		base = 2
		if spec.alternate {
			prefix = "0b"
		}
	case 'o':
		base = 8
		if spec.alternate {
			prefix = "0o"
		}
	case 'x', 'X':
		base = 16
		if spec.alternate {
			prefix = "0x"
			if typeCode == 'X' {
				prefix = "0X"
			}
		}
	case 'd':
	default:
		return "", newException(
			"ValueError",
			"Unknown format code '"+string(typeCode)+"' for object of type '"+typeName+"'",
		)
	}
	if spec.grouping == ',' && typeCode != 'd' {
		return "", newException(
			"ValueError",
			"Cannot specify ',' with '"+string(typeCode)+"'.",
		)
	}
	var magnitude big.Int
	magnitude.Abs(value)
	digits := magnitude.Text(base)
	if typeCode == 'X' {
		digits = strings.ToUpper(digits)
	}
	sign := integerSign(value.Sign() < 0, spec.sign)
	if spec.grouping != 0 {
		groupSize := 3
		if typeCode == 'b' || typeCode == 'o' || typeCode == 'x' || typeCode == 'X' {
			groupSize = 4
		}
		if spec.fill == "0" && spec.alignment == '=' && spec.width >= 0 {
			target := spec.width - len(sign) - len(prefix)
			digits = zeroPadIntegerGrouping(digits, groupSize, target)
		}
		digits = groupIntegerDigits(digits, spec.grouping, groupSize)
	}
	return padInteger(sign, prefix, digits, spec)
}

// formatIntegerCharacter validates c-specific options, preserves surrogate
// code points as WTF-8, and applies the shared numeric padding rules.
func formatIntegerCharacter(value *big.Int, spec integerFormatSpec) (string, *Exception) {
	if spec.sign != 0 {
		return "", newException(
			"ValueError",
			"Sign not allowed with integer format specifier 'c'",
		)
	}
	if spec.alternate {
		return "", newException(
			"ValueError",
			"Alternate form (#) not allowed with integer format specifier 'c'",
		)
	}
	if spec.grouping != 0 {
		return "", newException(
			"ValueError",
			"Cannot specify '"+string(spec.grouping)+"' with 'c'.",
		)
	}
	if !value.IsInt64() || value.Sign() < 0 || value.Int64() > 0x10ffff {
		return "", newException("OverflowError", "%c arg not in range(0x110000)")
	}
	codePoint := value.Int64()
	character := string(rune(codePoint))
	if codePoint >= 0xd800 && codePoint <= 0xdfff {
		character = string([]byte{
			byte(0xe0 | codePoint>>12),
			byte(0x80 | codePoint>>6&0x3f),
			byte(0x80 | codePoint&0x3f),
		})
	}
	return padInteger("", "", character, spec)
}

func integerSign(negative bool, option byte) string {
	if negative {
		return "-"
	}
	switch option {
	case '+':
		return "+"
	case ' ':
		return " "
	default:
		return ""
	}
}

// padInteger applies code-point fill around the sign, base prefix, and body.
func padInteger(sign, prefix, body string, spec integerFormatSpec) (string, *Exception) {
	contentWidth := len(stringCodepointOffsets(sign+prefix+body)) - 1
	padding := spec.width - contentWidth
	if padding <= 0 {
		return sign + prefix + body, nil
	}
	maxInt := int(^uint(0) >> 1)
	if len(spec.fill) != 0 && padding > maxInt/len(spec.fill) {
		return "", newException("ValueError", "format specifier is too large")
	}
	left, middle, right := 0, 0, 0
	switch spec.alignment {
	case '<':
		right = padding
	case '>':
		left = padding
	case '=':
		middle = padding
	case '^':
		left = padding / 2
		right = padding - left
	}
	fill := spec.fill
	return strings.Repeat(fill, left) + sign + prefix +
		strings.Repeat(fill, middle) + body + strings.Repeat(fill, right), nil
}

// parseIntegerFormatSpec accepts integer sign, prefix, zero padding, width,
// grouping, alignment, and presentation type fields.
func parseIntegerFormatSpec(text, typeName string) (integerFormatSpec, *Exception) {
	spec := integerFormatSpec{fill: " ", alignment: '>', width: -1}
	position := 0
	if text != "" {
		_, firstSize, _ := decodeStringRune(text)
		if firstSize < len(text) && isFormatAlignment(text[firstSize]) {
			spec.fill = text[:firstSize]
			spec.alignment = text[firstSize]
			spec.explicitAlignment = true
			position = firstSize + 1
		} else if firstSize == 1 && isFormatAlignment(text[0]) {
			spec.alignment = text[0]
			spec.explicitAlignment = true
			position = 1
		}
	}
	if position < len(text) && strings.ContainsRune("+- ", rune(text[position])) {
		spec.sign = text[position]
		position++
	}
	if position < len(text) && text[position] == 'z' {
		return integerFormatSpec{}, newException(
			"ValueError",
			"Negative zero coercion (z) not allowed in integer format specifier",
		)
	}
	if position < len(text) && text[position] == '#' {
		spec.alternate = true
		position++
	}
	if position < len(text) && text[position] == '0' {
		spec.zero = true
		position++
		if spec.fill == " " {
			spec.fill = "0"
			if !spec.explicitAlignment {
				spec.alignment = '='
			}
		}
	}
	var exception *Exception
	spec.width, position, exception = parseFormatNumber(text, position)
	if exception != nil {
		return integerFormatSpec{}, exception
	}
	if position < len(text) && (text[position] == ',' || text[position] == '_') {
		spec.grouping = text[position]
		position++
	}
	if position < len(text) && text[position] == '.' {
		return integerFormatSpec{}, newException(
			"ValueError",
			"Precision not allowed in integer format specifier",
		)
	}
	remaining := text[position:]
	if remaining == "" {
		return spec, nil
	}
	_, size, _ := decodeStringRune(remaining)
	if size != len(remaining) {
		return integerFormatSpec{}, newException(
			"ValueError",
			"Invalid format specifier "+quoteString(text)+
				" for object of type '"+typeName+"'",
		)
	}
	if size != 1 {
		return integerFormatSpec{}, newException(
			"ValueError",
			"Unknown format code '"+remaining+"' for object of type '"+typeName+"'",
		)
	}
	spec.typeCode = remaining[0]
	return spec, nil
}

// zeroPadIntegerGrouping prepends the fewest zeros whose grouped width reaches
// the requested digit field without counting sign or base prefix bytes.
func zeroPadIntegerGrouping(digits string, groupSize, target int) string {
	length := len(digits)
	if target > length {
		candidate := target - target/(groupSize+1)
		if candidate > length {
			length = candidate
		}
		for length+(length-1)/groupSize < target {
			length++
		}
		for length > len(digits) && length-1+(length-2)/groupSize >= target {
			length--
		}
	}
	return strings.Repeat("0", length-len(digits)) + digits
}

func groupIntegerDigits(digits string, separator byte, size int) string {
	if len(digits) <= size {
		return digits
	}
	first := len(digits) % size
	if first == 0 {
		first = size
	}
	var builder strings.Builder
	builder.Grow(len(digits) + (len(digits)-1)/size)
	builder.WriteString(digits[:first])
	for position := first; position < len(digits); position += size {
		builder.WriteByte(separator)
		builder.WriteString(digits[position : position+size])
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
