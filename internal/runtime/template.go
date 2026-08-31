package runtime

import (
	"fmt"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type interpolationValue struct {
	value      Value
	expression *stringValue
	conversion Value
	formatSpec *stringValue
}

func (*interpolationValue) TypeName() string { return "string.templatelib.Interpolation" }
func (value *interpolationValue) Repr() string {
	return "Interpolation(" + value.value.Repr() + ", " + value.expression.Repr() +
		", " + value.conversion.Repr() + ", " + value.formatSpec.Repr() + ")"
}
func (*interpolationValue) isValue() {}

type templateValue struct {
	strings        *tupleValue
	interpolations *tupleValue
}

func (*templateValue) TypeName() string { return "string.templatelib.Template" }
func (value *templateValue) Repr() string {
	return "Template(strings=" + value.strings.Repr() + ", interpolations=" +
		value.interpolations.Repr() + ")"
}
func (*templateValue) isValue() {}

type templateIterator struct {
	template *templateValue
	part     int
}

func (*templateIterator) TypeName() string { return "string.templatelib.TemplateIter" }
func (iterator *templateIterator) Repr() string {
	return "<" + iterator.TypeName() + " object>"
}
func (*templateIterator) isValue() {}

func (iterator *templateIterator) next() (Value, bool, *Exception) {
	lastPart := len(iterator.template.interpolations.elements) * 2
	for iterator.part <= lastPart {
		part := iterator.part
		iterator.part++
		if part%2 != 0 {
			return iterator.template.interpolations.elements[part/2], true, nil
		}
		literal := iterator.template.strings.elements[part/2].(*stringValue)
		if literal.value != "" {
			return literal, true, nil
		}
	}
	return nil, false, nil
}

// templateBinary handles Template's exact addition contract before generic
// numeric or user-object dispatch, including the asymmetric mixed-type errors.
func templateBinary(left, right Value, operand uint32) (Value, *Exception, bool) {
	leftTemplate, leftIsTemplate := left.(*templateValue)
	rightTemplate, rightIsTemplate := right.(*templateValue)
	if operand != bytecode.BinaryAdd {
		return nil, nil, false
	}
	if leftIsTemplate {
		if !rightIsTemplate {
			return nil, newException(
				"TypeError",
				"can only concatenate string.templatelib.Template (not \""+
					right.TypeName()+"\") to string.templatelib.Template",
			), true
		}
		return concatenateTemplates(leftTemplate, rightTemplate), nil, true
	}
	if _, leftIsString := left.(*stringValue); leftIsString && rightIsTemplate {
		return nil, newException(
			"TypeError",
			"can only concatenate str (not \"string.templatelib.Template\") to str",
		), true
	}
	return nil, nil, false
}

func concatenateTemplates(left, right *templateValue) *templateValue {
	leftStrings := left.strings.elements
	rightStrings := right.strings.elements
	strings := make([]Value, 0, len(leftStrings)+len(rightStrings)-1)
	strings = append(strings, leftStrings[:len(leftStrings)-1]...)
	boundary := leftStrings[len(leftStrings)-1].(*stringValue).value +
		rightStrings[0].(*stringValue).value
	strings = append(strings, &stringValue{value: boundary})
	strings = append(strings, rightStrings[1:]...)

	interpolations := make([]Value, 0,
		len(left.interpolations.elements)+len(right.interpolations.elements))
	interpolations = append(interpolations, left.interpolations.elements...)
	interpolations = append(interpolations, right.interpolations.elements...)
	return &templateValue{
		strings:        &tupleValue{elements: strings},
		interpolations: &tupleValue{elements: interpolations},
	}
}

func newTemplateLibraryModules() (*Module, *Module) {
	packageGlobals := newNamespace()
	packageGlobals.values["__name__"] = &stringValue{value: "string"}
	packageGlobals.values["__package__"] = &stringValue{value: "string"}
	packageGlobals.values["__path__"] = &listValue{}

	libraryGlobals := newNamespace()
	libraryGlobals.values["__name__"] = &stringValue{value: "string.templatelib"}
	libraryGlobals.values["__package__"] = &stringValue{value: "string"}
	libraryGlobals.values["Template"] = &builtinFunctionValue{
		name: "Template",
		call: builtinTemplate,
	}
	libraryGlobals.values["Interpolation"] = &builtinFunctionValue{
		name: "Interpolation",
		call: builtinInterpolation,
	}
	library := &Module{name: "string.templatelib", globals: libraryGlobals}
	packageGlobals.values["templatelib"] = library
	return &Module{
		name:            "string",
		globals:         packageGlobals,
		isPackage:       true,
		searchLocations: []string{},
	}, library
}

// builtinTemplate folds adjacent string arguments and inserts empty boundary
// strings around exact Interpolation values.
func builtinTemplate(arguments []Value, keywords *dictValue) (Value, *Exception) {
	if keywords != nil && len(keywords.entries) != 0 {
		return nil, newException(
			"TypeError",
			"Template.__new__ only accepts *args arguments",
		)
	}
	strings := make([]Value, 0, len(arguments)+1)
	interpolations := make([]Value, 0, len(arguments))
	literal := ""
	for _, argument := range arguments {
		switch argument := argument.(type) {
		case *stringValue:
			literal += argument.value
		case *interpolationValue:
			strings = append(strings, &stringValue{value: literal})
			interpolations = append(interpolations, argument)
			literal = ""
		default:
			return nil, newException(
				"TypeError",
				"Template.__new__ *args need to be of type 'str' or "+
					"'Interpolation', got "+argument.TypeName(),
			)
		}
	}
	strings = append(strings, &stringValue{value: literal})
	return &templateValue{
		strings:        &tupleValue{elements: strings},
		interpolations: &tupleValue{elements: interpolations},
	}, nil
}

// builtinInterpolation binds the four constructor fields and checks the two
// string metadata values plus the optional conversion marker.
func builtinInterpolation(arguments []Value, keywords *dictValue) (Value, *Exception) {
	values, exception := bindInterpolationArguments(arguments, keywords)
	if exception != nil {
		return nil, exception
	}
	expression, ok := values[1].(*stringValue)
	if !ok {
		return nil, interpolationArgumentTypeError("expression", "str", values[1])
	}
	conversion := values[2]
	if conversion != None {
		conversionText, ok := conversion.(*stringValue)
		if !ok {
			return nil, interpolationArgumentTypeError("conversion", "str", conversion)
		}
		if conversionText.value != "s" && conversionText.value != "a" &&
			conversionText.value != "r" {
			return nil, newException(
				"ValueError",
				"Interpolation() argument 'conversion' must be one of 's', 'a' or 'r'",
			)
		}
	}
	formatSpec, ok := values[3].(*stringValue)
	if !ok {
		return nil, interpolationArgumentTypeError("format_spec", "str", values[3])
	}
	return &interpolationValue{
		value:      values[0],
		expression: expression,
		conversion: conversion,
		formatSpec: formatSpec,
	}, nil
}

// bindInterpolationArguments combines positional and named constructor fields,
// rejects duplicates and unknown names, and supplies CPython's three defaults.
func bindInterpolationArguments(
	arguments []Value,
	keywords *dictValue,
) ([]Value, *Exception) {
	if len(arguments) > 4 {
		return nil, newException(
			"TypeError",
			fmt.Sprintf("Interpolation expected at most 4 arguments, got %d", len(arguments)),
		)
	}
	values := make([]Value, 4)
	copy(values, arguments)
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if !ok {
				return nil, newException("TypeError", "Interpolation() keywords must be strings")
			}
			index := interpolationArgumentIndex(name.value)
			if index < 0 {
				return nil, newException(
					"TypeError",
					"Interpolation() got an unexpected keyword argument '"+name.value+"'",
				)
			}
			if index < len(arguments) || values[index] != nil {
				return nil, newException(
					"TypeError",
					"Interpolation() got multiple values for argument '"+name.value+"'",
				)
			}
			values[index] = entry.value
		}
	}
	if values[0] == nil {
		return nil, newException(
			"TypeError",
			"Interpolation() missing required argument 'value' (pos 1)",
		)
	}
	if values[1] == nil {
		values[1] = &stringValue{value: ""}
	}
	if values[2] == nil {
		values[2] = None
	}
	if values[3] == nil {
		values[3] = &stringValue{value: ""}
	}
	return values, nil
}

func interpolationArgumentIndex(name string) int {
	switch name {
	case "value":
		return 0
	case "expression":
		return 1
	case "conversion":
		return 2
	case "format_spec":
		return 3
	default:
		return -1
	}
}

func interpolationArgumentTypeError(name, expected string, value Value) *Exception {
	return newException(
		"TypeError",
		"Interpolation() argument '"+name+"' must be "+expected+", not "+value.TypeName(),
	)
}

// executeBuildInterpolation validates compiler-created metadata and retains the
// evaluated value without applying its conversion or format string.
func executeBuildInterpolation(
	frame *frame,
	instruction int,
	operand uint32,
) (instructionOutcome, error) {
	conversion, hasFormat, ok := bytecode.InterpolationOperand(operand)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			fmt.Sprintf("unsupported BUILD_INTERPOLATION operand %d", operand),
		)
	}
	formatSpec := &stringValue{value: ""}
	if hasFormat {
		value, found := frame.pop()
		if !found {
			return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
		}
		var valid bool
		formatSpec, valid = value.(*stringValue)
		if !valid {
			return instructionOutcome{}, frame.failure(
				instruction,
				"template interpolation format is not a string",
			)
		}
	}
	expressionValue, found := frame.pop()
	if !found {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	expression, valid := expressionValue.(*stringValue)
	if !valid {
		return instructionOutcome{}, frame.failure(
			instruction,
			"template interpolation expression is not a string",
		)
	}
	value, found := frame.pop()
	if !found {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	conversionValue := Value(None)
	if conversion != 0 {
		conversionText := "s"
		if conversion == bytecode.ConversionRepr {
			conversionText = "r"
		} else if conversion == bytecode.ConversionASCII {
			conversionText = "a"
		}
		conversionValue = &stringValue{value: conversionText}
	}
	return pushOutcome(frame, instruction, &interpolationValue{
		value:      value,
		expression: expression,
		conversion: conversionValue,
		formatSpec: formatSpec,
	})
}

// executeBuildTemplate accepts only the compiler's parallel tuple layout and
// leaves one immutable template value on the operand stack.
func executeBuildTemplate(frame *frame, instruction int) (instructionOutcome, error) {
	interpolationsValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	stringsValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	strings, ok := stringsValue.(*tupleValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"template strings payload is not a tuple",
		)
	}
	interpolations, ok := interpolationsValue.(*tupleValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"template interpolations payload is not a tuple",
		)
	}
	if len(strings.elements) != len(interpolations.elements)+1 {
		return instructionOutcome{}, frame.failure(
			instruction,
			"template requires one more string than interpolation",
		)
	}
	for _, value := range strings.elements {
		if _, valid := value.(*stringValue); !valid {
			return instructionOutcome{}, frame.failure(
				instruction,
				"template strings payload contains a non-string value",
			)
		}
	}
	for _, value := range interpolations.elements {
		if _, valid := value.(*interpolationValue); !valid {
			return instructionOutcome{}, frame.failure(
				instruction,
				"template interpolations payload contains a non-interpolation value",
			)
		}
	}
	return pushOutcome(frame, instruction, &templateValue{
		strings:        strings,
		interpolations: interpolations,
	})
}

func executeTemplateAttributeLoad(
	frame *frame,
	instruction int,
	template *templateValue,
	name string,
) (instructionOutcome, error) {
	switch name {
	case "strings":
		return pushOutcome(frame, instruction, template.strings)
	case "interpolations":
		return pushOutcome(frame, instruction, template.interpolations)
	case "values":
		values := make([]Value, len(template.interpolations.elements))
		for index, value := range template.interpolations.elements {
			values[index] = value.(*interpolationValue).value
		}
		return pushOutcome(frame, instruction, &tupleValue{elements: values})
	default:
		return missingTemplateAttribute(template.TypeName(), name), nil
	}
}

func executeInterpolationAttributeLoad(
	frame *frame,
	instruction int,
	interpolation *interpolationValue,
	name string,
) (instructionOutcome, error) {
	switch name {
	case "value":
		return pushOutcome(frame, instruction, interpolation.value)
	case "expression":
		return pushOutcome(frame, instruction, interpolation.expression)
	case "conversion":
		return pushOutcome(frame, instruction, interpolation.conversion)
	case "format_spec":
		return pushOutcome(frame, instruction, interpolation.formatSpec)
	default:
		return missingTemplateAttribute(interpolation.TypeName(), name), nil
	}
}

func missingTemplateAttribute(typeName, name string) instructionOutcome {
	return instructionOutcome{
		kind: raised,
		exception: newException(
			"AttributeError",
			"'"+typeName+"' object has no attribute '"+name+"'",
		),
	}
}

var _ Value = (*templateValue)(nil)
var _ Value = (*interpolationValue)(nil)
