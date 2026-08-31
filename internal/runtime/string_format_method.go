package runtime

import (
	"strconv"
	"strings"
)

type stringFormatMethod struct {
	value *stringValue
}

type stringFormatCall struct {
	instruction int
	format      string
	arguments   []Value
	cursor      int
	next        int
	conversion  byte
	builder     strings.Builder
}

func (*stringFormatMethod) TypeName() string { return "builtin_function_or_method" }
func (*stringFormatMethod) Repr() string {
	return "<built-in method format of str object>"
}
func (*stringFormatMethod) isValue() {}

// continueStringFormat copies literal text and escaped braces until one field
// needs conversion or the complete result can be returned.
func continueStringFormat(
	frame *frame,
	call *stringFormatCall,
) (instructionOutcome, error) {
	for call.cursor < len(call.format) {
		current := call.format[call.cursor]
		switch current {
		case '{':
			if call.cursor+1 < len(call.format) && call.format[call.cursor+1] == '{' {
				call.builder.WriteByte('{')
				call.cursor += 2
				continue
			}
			end := strings.IndexByte(call.format[call.cursor+1:], '}')
			if end < 0 {
				return raiseOutcome(newException(
					"ValueError",
					"Single '{' encountered in format string",
				)), nil
			}
			end += call.cursor + 1
			field := call.format[call.cursor+1 : end]
			conversion, exception := parseAutomaticFormatField(field)
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if call.next >= len(call.arguments) {
				return raiseOutcome(newException(
					"IndexError",
					"Replacement index "+strconv.Itoa(call.next)+
						" out of range for positional args tuple",
				)), nil
			}
			value := call.arguments[call.next]
			call.next++
			call.cursor = end + 1
			call.conversion = conversion
			return startStringFormatValue(frame, call, value)
		case '}':
			if call.cursor+1 < len(call.format) && call.format[call.cursor+1] == '}' {
				call.builder.WriteByte('}')
				call.cursor += 2
				continue
			}
			return raiseOutcome(newException(
				"ValueError",
				"Single '}' encountered in format string",
			)), nil
		default:
			call.builder.WriteByte(current)
			call.cursor++
		}
	}
	return pushOutcome(frame, call.instruction, &stringValue{value: call.builder.String()})
}

// parseAutomaticFormatField accepts the empty automatic field and three
// conversion markers, then reports each deferred field family explicitly.
func parseAutomaticFormatField(field string) (byte, *Exception) {
	if strings.Contains(field, ":") {
		return 0, newException(
			"NotImplementedError",
			"str.format specifications are not supported",
		)
	}
	if field == "" {
		return 0, nil
	}
	if field[0] == '!' {
		if len(field) == 2 && strings.ContainsRune("sra", rune(field[1])) {
			return field[1], nil
		}
		conversion := field[1:]
		return 0, newException(
			"ValueError",
			"Unknown conversion specifier "+conversion,
		)
	}
	if field[0] >= '0' && field[0] <= '9' {
		return 0, newException(
			"NotImplementedError",
			"numbered str.format fields are not supported",
		)
	}
	return 0, newException(
		"NotImplementedError",
		"named str.format fields are not supported",
	)
}

// startStringFormatValue selects string or representation conversion, rejects
// custom formatting, and attaches the format state to a suspended user call.
func startStringFormatValue(
	frame *frame,
	call *stringFormatCall,
	value Value,
) (instructionOutcome, error) {
	var outcome instructionOutcome
	var err error
	switch call.conversion {
	case 'r', 'a':
		outcome, err = executeRepresentation(frame, call.instruction, value)
	default:
		if call.conversion == 0 {
			if instance, ok := value.(*instanceValue); ok {
				if _, found := lookupInstanceSpecial(instance, "__format__"); found {
					return raiseOutcome(newException(
						"NotImplementedError",
						"custom __format__ methods are not supported by str.format",
					)), nil
				}
			}
		}
		outcome, err = executeString(frame, call.instruction, value)
	}
	if err != nil || outcome.kind != advance {
		if outcome.kind == called && outcome.frame.representation != nil {
			outcome.frame.representation.formatting = call
		}
		return outcome, err
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			call.instruction,
			"string format conversion returned without a value",
		)
	}
	text, ok := result.(*stringValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			call.instruction,
			"string format conversion did not produce a string",
		)
	}
	return finishStringFormatValue(frame, call, text)
}

func finishStringFormatValue(
	frame *frame,
	call *stringFormatCall,
	value *stringValue,
) (instructionOutcome, error) {
	text := value.value
	if call.conversion == 'a' {
		text = asciiRepresentation(text)
	}
	call.builder.WriteString(text)
	call.conversion = 0
	return continueStringFormat(frame, call)
}
