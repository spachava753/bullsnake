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
	numbering   uint8
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
			field, exception := parsePositionalFormatField(call.format[call.cursor+1 : end])
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if exception := call.selectField(&field); exception != nil {
				return raiseOutcome(exception), nil
			}
			if field.index >= len(call.arguments) {
				return raiseOutcome(newException(
					"IndexError",
					"Replacement index "+strconv.Itoa(field.index)+
						" out of range for positional args tuple",
				)), nil
			}
			value := call.arguments[field.index]
			call.cursor = end + 1
			call.conversion = field.conversion
			return continueStringFormatAttributes(frame, call, value, field.attributes)
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

type positionalFormatField struct {
	index      int
	attributes []string
	conversion byte
}

// parsePositionalFormatField accepts automatic or decimal positions followed by
// attribute paths and optional conversion. Other field families remain explicit.
func parsePositionalFormatField(text string) (positionalFormatField, *Exception) {
	field := positionalFormatField{index: -1}
	if strings.Contains(text, ":") {
		return field, newException("NotImplementedError", "str.format specifications are not supported")
	}
	if before, conversion, found := strings.Cut(text, "!"); found {
		if len(conversion) != 1 || !strings.ContainsRune("sra", rune(conversion[0])) {
			return field, newException("ValueError", "Unknown conversion specifier "+conversion)
		}
		text, field.conversion = before, conversion[0]
	}
	if strings.Contains(text, "[") {
		return field, newException("NotImplementedError", "indexed str.format fields are not supported")
	}
	parts := strings.Split(text, ".")
	if parts[0] != "" {
		for _, char := range parts[0] {
			if char < '0' || char > '9' {
				return field, newException("NotImplementedError", "named str.format fields are not supported")
			}
		}
		index, err := strconv.Atoi(parts[0])
		if err != nil {
			return field, newException("ValueError", "Too many decimal digits in format string")
		}
		field.index = index
	}
	field.attributes = parts[1:]
	for _, name := range field.attributes {
		if name == "" {
			return field, newException("ValueError", "Empty attribute in format string")
		}
	}
	return field, nil
}

func (call *stringFormatCall) selectField(field *positionalFormatField) *Exception {
	if field.index < 0 {
		if call.numbering == 2 {
			return newException("ValueError", "cannot switch from manual field specification to automatic field numbering")
		}
		call.numbering = 1
		field.index = call.next
		call.next++
	} else {
		if call.numbering == 1 {
			return newException("ValueError", "cannot switch from automatic field numbering to manual field specification")
		}
		call.numbering = 2
	}
	return nil
}

// continueStringFormatAttributes resolves each real attribute through the VM,
// then converts the resulting value. Immediate reads stay in a Go loop.
func continueStringFormatAttributes(caller *frame, call *stringFormatCall, value Value, attributes []string) (instructionOutcome, error) {
	for len(attributes) != 0 {
		name, remaining := attributes[0], attributes[1:]
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := executeDynamicAttributeLoad(caller, call.instruction, value, name)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if suspended {
				return continueStringFormatAttributes(current, call, result, remaining)
			}
			return pushOutcome(current, call.instruction, result)
		})
		if err != nil || outcome.kind != advance {
			return outcome, err
		}
		value, _ = caller.pop()
		attributes = remaining
	}
	return startStringFormatValue(caller, call, value)
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
