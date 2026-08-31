package runtime

import "strconv"

// executeMatchClass validates a class pattern and returns extracted attributes
// plus a match flag. Missing attributes are mismatches rather than exceptions.
func executeMatchClass(
	frame *frame,
	instruction int,
	positionalCount int,
) (instructionOutcome, error) {
	keywordNamesValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	patternClass, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	subject, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	keywordNames, ok := keywordNamesValue.(*tupleValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"MATCH_CLASS keyword names are not a tuple",
		)
	}

	className, matched, matchArgs, hasMatchArgs, validClass := classPatternInfo(
		subject,
		patternClass,
	)
	if !validClass {
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "called match pattern must be a class"),
		}, nil
	}
	if !matched {
		return pushClassMatchResult(frame, instruction, nil, false)
	}

	attributeNames := make([]string, 0, positionalCount+len(keywordNames.elements))
	if positionalCount != 0 {
		if !hasMatchArgs {
			matchArgs = &tupleValue{}
		}
		if matchArgs == nil {
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"TypeError",
					className+".__match_args__ must be a tuple (got "+
						classPatternMatchArgsType(patternClass)+")",
				),
			}, nil
		}
		if positionalCount > len(matchArgs.elements) {
			plural := "s"
			if len(matchArgs.elements) == 1 {
				plural = ""
			}
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"TypeError",
					className+"() accepts "+strconv.Itoa(len(matchArgs.elements))+
						" positional sub-pattern"+plural+" ("+
						strconv.Itoa(positionalCount)+" given)",
				),
			}, nil
		}
		for _, value := range matchArgs.elements[:positionalCount] {
			name, isString := value.(*stringValue)
			if !isString {
				return instructionOutcome{
					kind: raised,
					exception: newException(
						"TypeError",
						"__match_args__ elements must be strings (got "+
							value.TypeName()+")",
					),
				}, nil
			}
			attributeNames = append(attributeNames, name.value)
		}
	}
	for _, value := range keywordNames.elements {
		name, isString := value.(*stringValue)
		if !isString {
			return instructionOutcome{}, frame.failure(
				instruction,
				"MATCH_CLASS keyword name is not a string",
			)
		}
		attributeNames = append(attributeNames, name.value)
	}

	seen := make(map[string]struct{}, len(attributeNames))
	attributes := make([]Value, 0, len(attributeNames))
	for _, name := range attributeNames {
		if _, duplicate := seen[name]; duplicate {
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"TypeError",
					className+"() got multiple sub-patterns for attribute "+
						(&stringValue{value: name}).Repr(),
				),
			}, nil
		}
		seen[name] = struct{}{}
		value, found := classPatternAttribute(subject, name)
		if !found {
			return pushClassMatchResult(frame, instruction, nil, false)
		}
		attributes = append(attributes, value)
	}
	return pushClassMatchResult(frame, instruction, attributes, true)
}

// classPatternInfo validates the supported class objects, tests their current
// ancestry rules, and returns inherited positional-pattern metadata.
func classPatternInfo(
	subject Value,
	patternClass Value,
) (name string, matched bool, matchArgs *tupleValue, hasMatchArgs bool, valid bool) {
	switch class := patternClass.(type) {
	case *typeValue:
		matched = false
		switch subject := subject.(type) {
		case *instanceValue:
			matched = subject.class.isSubclassOf(class)
		case *Exception:
			matched = exceptionMatchesClass(subject, class)
		}
		matchArgsValue, found := class.lookup("__match_args__")
		if !found {
			return class.name, matched, nil, false, true
		}
		matchArgs, tuple := matchArgsValue.(*tupleValue)
		if !tuple {
			return class.name, matched, nil, true, true
		}
		return class.name, matched, matchArgs, true, true
	case *exceptionTypeValue:
		exception, isException := subject.(*Exception)
		return class.name,
			isException && exceptionMatchesClass(exception, class),
			nil,
			false,
			true
	default:
		return "", false, nil, false, false
	}
}

func classPatternMatchArgsType(patternClass Value) string {
	class, ok := patternClass.(*typeValue)
	if !ok {
		return "NoneType"
	}
	value, found := class.lookup("__match_args__")
	if !found {
		return "NoneType"
	}
	return value.TypeName()
}

// classPatternAttribute follows the runtime's current instance lookup rules but
// reports a missing field as a pattern mismatch instead of AttributeError.
func classPatternAttribute(subject Value, name string) (Value, bool) {
	switch subject := subject.(type) {
	case *instanceValue:
		value, found := subject.attributes.get(name)
		fromClass := !found
		if !found {
			value, found = subject.class.lookup(name)
		}
		if !found {
			return nil, false
		}
		if fromClass {
			if bound, descriptor := bindMethodDescriptor(value, subject.class); descriptor {
				value = bound
			} else if function, bind := value.(*functionValue); bind {
				value = &boundMethodValue{callable: function, self: subject}
			}
		}
		return value, true
	case *Exception:
		return subject.attribute(name)
	default:
		return nil, false
	}
}

func pushClassMatchResult(
	frame *frame,
	instruction int,
	attributes []Value,
	matched bool,
) (instructionOutcome, error) {
	var result Value = None
	if matched {
		result = &tupleValue{elements: attributes}
	}
	if !frame.push(result) {
		return instructionOutcome{}, frame.failure(instruction, "operand stack overflow")
	}
	flag := falseSingleton
	if matched {
		flag = trueSingleton
	}
	return pushOutcome(frame, instruction, flag)
}
