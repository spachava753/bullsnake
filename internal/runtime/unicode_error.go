package runtime

import (
	"fmt"
	"math/big"
)

var (
	unicodeErrorType       = &exceptionTypeValue{name: "UnicodeError", base: valueErrorType}
	unicodeEncodeErrorType = &exceptionTypeValue{name: "UnicodeEncodeError", base: unicodeErrorType}
	unicodeDecodeErrorType = &exceptionTypeValue{name: "UnicodeDecodeError", base: unicodeErrorType}
)

// newUnicodeError checks codec exception arguments and retains the source object
// and offsets. The initial stream adapters report one invalid unit at a time.
func newUnicodeError(class *exceptionTypeValue, arguments []Value) (*Exception, *Exception) {
	if len(arguments) != 5 {
		return nil, newException("TypeError", class.name+" requires five arguments")
	}
	for _, index := range []int{0, 4} {
		if _, ok := arguments[index].(*stringValue); !ok {
			return nil, newException("TypeError", "encoding and reason must be strings")
		}
	}
	for _, index := range []int{2, 3} {
		number, ok := integerOperand(arguments[index])
		if !ok {
			return nil, newException("TypeError", "start and end must be integers")
		}
		if !number.IsInt64() {
			return nil, newException("OverflowError", "codec error offset does not fit an index-sized integer")
		}
	}
	if class == unicodeEncodeErrorType {
		if _, ok := arguments[1].(*stringValue); !ok {
			return nil, newException("TypeError", "encoding error object must be str")
		}
	} else {
		if _, ok := arguments[1].(*bytesValue); !ok {
			return nil, newException("TypeError", "decoding error object must be bytes")
		}
	}
	exception := newExceptionOfType(class, "")
	exception.setArguments(arguments)
	exception.fields = newNamespace()
	for index, name := range []string{"encoding", "object", "start", "end", "reason"} {
		exception.fields.values[name] = arguments[index]
	}
	for _, name := range []string{"start", "end"} {
		number, _ := integerOperand(exception.fields.values[name])
		exception.fields.values[name] = integerFromInt64(number.Int64())
	}
	exception.message = unicodeErrorMessage(class, arguments)
	return exception, nil
}

func streamUnicodeError(class *exceptionTypeValue, object Value, start int64, reason string) *Exception {
	exception, _ := newUnicodeError(class, []Value{
		&stringValue{value: "utf-8"}, object, integerFromInt64(start), integerFromInt64(start + 1), &stringValue{value: reason},
	})
	return exception
}

// unicodeErrorMessage follows the single-unit and range formats of the pinned
// codec exceptions, using character offsets for encoding and bytes for decoding.
func unicodeErrorMessage(class *exceptionTypeValue, arguments []Value) string {
	start, _ := integerOperand(arguments[2])
	end, _ := integerOperand(arguments[3])
	var last big.Int
	last.Sub(&end, big.NewInt(1))
	prefix := "'" + valueText(arguments[0]) + "' codec can't "
	reason := valueText(arguments[4])
	action := "decode bytes"
	if class == unicodeEncodeErrorType {
		action = "encode characters"
	}
	if start.IsInt64() && end.IsInt64() && start.Sign() >= 0 && end.Int64() == start.Int64()+1 {
		index := start.Int64()
		if object, ok := arguments[1].(*bytesValue); ok && index < int64(len(object.value)) {
			return fmt.Sprintf("%sdecode byte 0x%02x in position %d: %s", prefix, object.value[index], index, reason)
		}
		if object, ok := arguments[1].(*stringValue); ok {
			offsets := stringCodepointOffsets(object.value)
			if index < int64(len(offsets)-1) {
				current, _, _ := decodeStringRune(object.value[offsets[index]:])
				escape := fmt.Sprintf("\\U%08x", current)
				if current <= 0xff {
					escape = fmt.Sprintf("\\x%02x", current)
				} else if current <= 0xffff {
					escape = fmt.Sprintf("\\u%04x", current)
				}
				return fmt.Sprintf("%sencode character '%s' in position %d: %s", prefix, escape, index, reason)
			}
		}
	}
	return fmt.Sprintf("%s%s in position %s-%s: %s", prefix, action, start.String(), last.String(), reason)
}
