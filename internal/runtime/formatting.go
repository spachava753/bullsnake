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
