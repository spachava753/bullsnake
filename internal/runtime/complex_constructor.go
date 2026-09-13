package runtime

import "strconv"

// executeComplexConstructor handles numeric components and a single __complex__
// conversion, preserving unsupported parsing and warning-dependent boundaries.
func executeComplexConstructor(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	real, imaginary, exception := complexArguments(arguments, keywords)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	if imaginary == nil {
		if value, ok := real.(*complexValue); ok {
			return pushOutcome(caller, instruction, value)
		}
		if value, ok := real.(*instanceValue); ok {
			if method, found := lookupInstanceSpecial(value, "__complex__"); found {
				return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
					return executeFunctionCall(caller, instruction, len(caller.stack), method, nil, nil)
				}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
					if exception != nil {
						return raiseOutcome(exception), nil
					}
					if _, ok := result.(*complexValue); !ok {
						return raiseOutcome(newException("TypeError", "__complex__ returned non-complex (type "+result.TypeName()+")")), nil
					}
					return pushOutcome(current, instruction, result)
				})
			}
		}
		if _, text := real.(*stringValue); text {
			return raiseOutcome(newException("NotImplementedError", "complex() string parsing is not supported")), nil
		}
	}
	r, exception := complexComponent(real, false)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	i := 0.0
	if imaginary != nil {
		i, exception = complexComponent(imaginary, true)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
	}
	return pushOutcome(caller, instruction, &complexValue{real: r, imaginary: i})
}

// complexArguments distinguishes an omitted imaginary component from an
// explicit zero and binds only the supported real/imag keyword spellings.
func complexArguments(arguments []Value, keywords *dictValue) (Value, Value, *Exception) {
	if len(arguments) > 2 {
		return nil, nil, newException("TypeError", "complex() takes at most 2 arguments ("+strconv.Itoa(len(arguments))+" given)")
	}
	real := Value(integerFromInt64(0))
	var imaginary Value
	if len(arguments) > 0 {
		real = arguments[0]
	}
	if len(arguments) > 1 {
		imaginary = arguments[1]
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			key, ok := entry.key.(*stringValue)
			if !ok {
				return nil, nil, newException("TypeError", "keywords must be strings")
			}
			index := 0
			if key.value == "imag" {
				index = 1
			} else if key.value != "real" {
				return nil, nil, newException("TypeError", "complex() got an unexpected keyword argument '"+key.value+"'")
			}
			if len(arguments) > index {
				return nil, nil, newException("TypeError", "argument for complex() given by name ('"+key.value+"') and position ("+strconv.Itoa(index+1)+")")
			}
			if index == 0 {
				real = entry.value
			} else {
				imaginary = entry.value
			}
		}
	}
	return real, imaginary, nil
}

// complexComponent converts native real numbers and separates invalid values
// from deferred Python conversion and deprecation-warning behavior.
func complexComponent(value Value, imaginary bool) (float64, *Exception) {
	if _, text := value.(*stringValue); text {
		message := "complex() can't take second arg if first is a string"
		if imaginary {
			message = "complex() second arg can't be a string"
		}
		return 0, newException("TypeError", message)
	}
	if number, exception, ok := numericFloat(value); ok {
		return number, exception
	}
	if _, complex := value.(*complexValue); complex {
		return 0, newException("NotImplementedError", "complex() deprecated complex-valued component arguments require warning support")
	}
	if instance, ok := value.(*instanceValue); ok {
		for _, name := range []string{"__complex__", "__float__", "__index__"} {
			if _, found := lookupInstanceSpecial(instance, name); found {
				return 0, newException("NotImplementedError", "complex() component conversion fallback is not supported")
			}
		}
	}
	argument := "first argument must be a string or a number"
	if imaginary {
		argument = "second argument must be a number"
	}
	return 0, newException("TypeError", "complex() "+argument+", not '"+value.TypeName()+"'")
}

// addComplexDescriptors exposes real components and the identity conversion
// method; read-only metadata never mutates the immutable numeric value.
func addComplexDescriptors(class *nativeTypeValue, dictionary *dictValue) {
	if class != complexNativeType {
		return
	}
	for _, name := range []string{"real", "imag"} {
		dictionary.set(&stringValue{value: name}, &nativeDataDescriptorValue{class: class, name: name, member: true, get: func(caller *frame, instruction int, self Value) (instructionOutcome, error) {
			number := self.(*complexValue).real
			if name == "imag" {
				number = self.(*complexValue).imaginary
			}
			return pushOutcome(caller, instruction, &floatValue{value: number})
		}})
	}
	dictionary.set(&stringValue{value: "__complex__"}, &nativeDescriptorValue{class: class, name: "__complex__", kind: nativeMethodDescriptor, call: func(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		if exception := checkNativeArguments("__complex__", arguments, keywords, 0, 0); exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, instruction, self)
	}})
}

func executeComplexAttribute(caller *frame, instruction int, value *complexValue, name string) (instructionOutcome, error) {
	if attribute, found := caller.runtime.nativeClassAttribute(complexNativeType, name); found {
		if descriptor, ok := attribute.(*nativeDataDescriptorValue); ok {
			return descriptor.load(caller, instruction, value)
		}
		return pushOutcome(caller, instruction, bindInstanceFunction(attribute, value))
	}
	return raiseOutcome(newException("AttributeError", "'complex' object has no attribute '"+name+"'")), nil
}
