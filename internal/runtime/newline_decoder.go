package runtime

import (
	"math/big"
	"strings"
)

type newlineDecoder struct {
	decoder   Value
	errors    Value
	pendingCR bool
	newlines  stringIOValue
}

func initializeNewlineDecoder(module *Module) {
	class := newIOClass("IncrementalNewlineDecoder", nil)
	module.globals.values[class.name] = class
	for _, name := range []string{"__init__", "decode", "getstate", "setstate", "reset"} {
		class.setAttribute(name, ioMethod(class, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			return executeNewlineDecoder(caller, instruction, self, name, arguments, keywords)
		}))
	}
	class.setAttribute("newlines", &propertyValue{doc: None, getter: ioMethod(class, "newlines", func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		return executeNewlineDecoder(caller, instruction, self, "newlines", arguments, keywords)
	})})
}

// executeNewlineDecoder binds boolean arguments through Python truth methods,
// then delegates codec calls using VM continuations rather than Go recursion.
func executeNewlineDecoder(caller *frame, instruction int, self *instanceValue, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if name == "__init__" || name == "decode" {
		names := []string{"input", "final"}
		defaults := []Value{nil, falseSingleton}
		if name == "__init__" {
			names = []string{"decoder", "translate", "errors"}
			defaults = []Value{nil, nil, &stringValue{value: "strict"}}
		}
		values, exception := bindIOArguments(name, arguments, keywords, names, defaults)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeBuiltinBool(caller, instruction, len(caller.stack), []Value{values[1]}, nil)
		}, func(current *frame, truth Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if name == "__init__" {
				self.io.decoder = &newlineDecoder{decoder: values[0], errors: values[2], newlines: stringIOValue{universal: true, translate: truth == trueSingleton}}
				return pushOutcome(current, instruction, None)
			}
			return decodeNewlines(current, instruction, self.io.decoder, values[0], truth == trueSingleton)
		})
	}
	count := 0
	if name == "setstate" {
		count = 1
	}
	if exception := checkNativeArguments(name, arguments, keywords, count, count); exception != nil {
		return raiseOutcome(exception), nil
	}
	decoder := self.io.decoder
	if decoder == nil {
		return raiseOutcome(uninitializedDecoder()), nil
	}
	switch name {
	case "newlines":
		return pushOutcome(caller, instruction, decoder.newlines.newlines())
	case "reset":
		decoder.pendingCR, decoder.newlines.seen = false, 0
		if decoder.decoder != None {
			return executeMethodCall(caller, instruction, decoder.decoder, name, nil)
		}
		return pushOutcome(caller, instruction, None)
	case "setstate":
		buffer, flag, exception := newlineDecoderState(arguments[0], "state argument must be a tuple")
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		decoder.pendingCR = flag&1 != 0
		if decoder.decoder != None {
			state := &tupleValue{elements: []Value{buffer, &intValue{value: *new(big.Int).SetUint64(flag >> 1)}}}
			return executeMethodCall(caller, instruction, decoder.decoder, name, []Value{state})
		}
		return pushOutcome(caller, instruction, None)
	default:
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			if decoder.decoder != None {
				return executeMethodCall(caller, instruction, decoder.decoder, "getstate", nil)
			}
			return pushOutcome(caller, instruction, &tupleValue{elements: []Value{&bytesValue{}, integerFromInt64(0)}})
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			buffer, flag, exception := newlineDecoderState(result, "illegal decoder state")
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			flag <<= 1
			if decoder.pendingCR {
				flag |= 1
			}
			return pushOutcome(current, instruction, &tupleValue{elements: []Value{buffer, &intValue{value: *new(big.Int).SetUint64(flag)}}})
		})
	}
}

func uninitializedDecoder() *Exception {
	return newException("ValueError", "IncrementalNewlineDecoder.__init__() not called")
}

// newlineDecoderState accepts the codec's two-tuple and masks integer flags to
// unsigned 64 bits, matching CPython's K argument conversion including negatives.
func newlineDecoderState(value Value, message string) (Value, uint64, *Exception) {
	state, ok := value.(*tupleValue)
	if !ok || len(state.elements) != 2 {
		return nil, 0, newException("TypeError", message)
	}
	integer, ok := integerOperand(state.elements[1])
	if !ok {
		return nil, 0, newException("TypeError", "an integer is required")
	}
	modulus := new(big.Int).Lsh(big.NewInt(1), 64)
	integer.Mod(&integer, modulus)
	return state.elements[0], integer.Uint64(), nil
}

// decodeNewlines validates decoded text before changing state, carries a final
// CR across empty chunks, and preserves internal surrogate code points verbatim.
func decodeNewlines(caller *frame, instruction int, decoder *newlineDecoder, input Value, final bool) (instructionOutcome, error) {
	if decoder == nil {
		return raiseOutcome(uninitializedDecoder()), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		if decoder.decoder == None {
			return pushOutcome(caller, instruction, input)
		}
		return executeMethodCall(caller, instruction, decoder.decoder, "decode", []Value{input, booleanValue(final)})
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		text, ok := result.(*stringValue)
		if !ok {
			return raiseOutcome(newException("TypeError", "decoder should return a string result, not '"+result.TypeName()+"'")), nil
		}
		value := text.value
		if decoder.pendingCR && (final || value != "") {
			value, decoder.pendingCR = "\r"+value, false
		}
		if !final && strings.HasSuffix(value, "\r") {
			value, decoder.pendingCR = value[:len(value)-1], true
		}
		return pushOutcome(current, instruction, &stringValue{value: decoder.newlines.translateNewlines(value)})
	})
}
