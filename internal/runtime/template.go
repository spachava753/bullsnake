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
