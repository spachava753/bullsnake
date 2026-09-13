package runtime

import (
	"fmt"
	"strings"
)

type percentTextCall struct {
	instruction int
	format      string
	arguments   []Value
	cursor      int
	next        int
	mapping     bool
	builder     strings.Builder
}

func executePercentText(caller *frame, instruction int, format *stringValue, values Value) (instructionOutcome, error) {
	call := &percentTextCall{instruction: instruction, format: format.value}
	if tuple, ok := values.(*tupleValue); ok {
		call.arguments = tuple.elements
	} else {
		call.arguments = []Value{values}
		_, call.mapping = dictionaryStorage(values)
	}
	return call.advance(caller)
}

// advance parses one field at a time, so earlier conversion callbacks still run
// before later format errors. Immediate conversions use a bounded Go loop.
func (call *percentTextCall) advance(caller *frame) (instructionOutcome, error) {
	for call.cursor < len(call.format) {
		offset := strings.IndexByte(call.format[call.cursor:], '%')
		if offset < 0 {
			call.builder.WriteString(call.format[call.cursor:])
			call.cursor = len(call.format)
			break
		}
		call.builder.WriteString(call.format[call.cursor : call.cursor+offset])
		call.cursor += offset + 1
		if call.cursor == len(call.format) {
			return raiseOutcome(newException("ValueError", "incomplete format")), nil
		}
		conversion := call.format[call.cursor]
		call.cursor++
		if conversion == '%' {
			call.builder.WriteByte('%')
			continue
		}
		if call.next >= len(call.arguments) {
			return raiseOutcome(newException("TypeError", "not enough arguments for format string")), nil
		}
		if conversion != 's' && conversion != 'r' && conversion != 'a' {
			return raiseOutcome(call.unsupportedConversion(conversion)), nil
		}
		value := call.arguments[call.next]
		call.next++
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			var outcome instructionOutcome
			var err error
			if conversion == 's' {
				outcome, err = executeString(caller, call.instruction, value)
			} else {
				outcome, err = executeRepresentation(caller, call.instruction, value)
			}
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			text := result.(*stringValue).value
			if conversion == 'a' {
				text = asciiRepresentation(text)
			}
			call.builder.WriteString(text)
			if suspended {
				return call.advance(current)
			}
			return pushOutcome(current, call.instruction, None)
		})
		if err != nil || outcome.kind != advance {
			return outcome, err
		}
		caller.pop()
	}
	if call.next != len(call.arguments) && !call.mapping {
		return raiseOutcome(newException("TypeError", "not all arguments converted during string formatting")), nil
	}
	return pushOutcome(caller, call.instruction, &stringValue{value: call.builder.String()})
}

func (call *percentTextCall) unsupportedConversion(conversion byte) *Exception {
	if strings.ContainsRune("#0-+ .*(123456789diouxXeEfFgGc", rune(conversion)) {
		return newException("NotImplementedError", "percent formatting supports only positional %s, %r, %a, and %%")
	}
	character, _, _ := decodeStringRune(call.format[call.cursor-1:])
	index := len(stringCodepointOffsets(call.format[:call.cursor-1])) - 1
	return newException("ValueError", fmt.Sprintf("unsupported format character '%c' (0x%x) at index %d", character, character, index))
}

// addPercentTextDescriptors binds normal and reflected native remainder slots;
// a non-string reflected left operand is declined rather than coerced.
func addPercentTextDescriptors(class *nativeTypeValue, dictionary *dictValue) {
	if class != stringNativeType {
		return
	}
	for _, name := range []string{"__mod__", "__rmod__"} {
		dictionary.set(&stringValue{value: name}, &nativeDescriptorValue{class: class, name: name, call: func(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			if exception := checkNativeArguments(name, arguments, keywords, 1, 1); exception != nil {
				return raiseOutcome(exception), nil
			}
			if name == "__rmod__" {
				if format, ok := arguments[0].(*stringValue); ok {
					return executePercentText(caller, instruction, format, self)
				}
				return pushOutcome(caller, instruction, notImplementedSingleton)
			}
			return executePercentText(caller, instruction, self.(*stringValue), arguments[0])
		}})
	}
}
