package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

// percentFormat scans percent directives and consumes tuple or mapping arguments.
func percentFormat(format string, argument Value) (string, *Exception) {
	arguments := []Value{argument}
	if tuple, ok := argument.(*tupleValue); ok {
		arguments = tuple.elements
	} else if instance, ok := argument.(*instanceValue); ok && instance.tuple != nil {
		arguments = instance.tuple.elements
	}
	argumentIndex := 0
	mappingFormat := false
	var builder strings.Builder
	for index := 0; index < len(format); index++ {
		if format[index] != '%' {
			builder.WriteByte(format[index])
			continue
		}
		index++
		if index < len(format) && format[index] == '%' {
			builder.WriteByte('%')
			continue
		}
		var value Value
		if index < len(format) && format[index] == '(' {
			mappingFormat = true
			end := strings.IndexByte(format[index+1:], ')')
			if end < 0 {
				return "", newException("ValueError", "incomplete format key")
			}
			name := format[index+1 : index+1+end]
			dictionary, ok := argument.(*dictValue)
			if instance, instanceOK := argument.(*instanceValue); instanceOK && instance.mapping != nil {
				dictionary, ok = instance.mapping, true
			}
			if namespace, namespaceOK := argument.(*namespaceValue); namespaceOK {
				dictionary, ok = namespace.dictionary(), true
			}
			if !ok {
				return "", newException("TypeError", "format requires a mapping")
			}
			var found bool
			value, found, _ = dictionary.get(&stringValue{value: name})
			if !found {
				return "", newException("KeyError", (&stringValue{value: name}).Repr())
			}
			index += end + 2
		} else {
			if argumentIndex >= len(arguments) {
				return "", newException("TypeError", "not enough arguments for format string")
			}
			value = arguments[argumentIndex]
			argumentIndex++
		}
		specStart := index
		for index < len(format) && strings.ContainsRune("#0- +0123456789.*", rune(format[index])) {
			index++
		}
		if index >= len(format) {
			return "", newException("ValueError", "incomplete format")
		}
		text, exception := formatPercentValue(value, format[specStart:index], format[index])
		if exception != nil {
			return "", exception
		}
		builder.WriteString(text)
	}
	if !mappingFormat && argumentIndex < len(arguments) {
		return "", newException("TypeError", "not all arguments converted during string formatting")
	}
	return builder.String(), nil
}

// formatPercentValue renders one supported directive according to width and flags.
func formatPercentValue(value Value, specification string, conversion byte) (string, *Exception) {
	switch conversion {
	case 's':
		if text, ok := value.(*stringValue); ok {
			return applyStringWidth(text.value, specification), nil
		}
		return applyStringWidth(value.Repr(), specification), nil
	case 'r', 'a':
		return applyStringWidth(value.Repr(), specification), nil
	case 'd', 'i', 'u':
		integer, ok := integerOperand(value)
		if !ok {
			return "", newException("TypeError", "%d format: a real number is required")
		}
		return integer.String(), nil
	case 'x', 'X':
		integer, ok := integerOperand(value)
		if !ok {
			return "", newException("TypeError", "%x format: an integer is required")
		}
		text := integer.Text(16)
		if conversion == 'X' {
			text = strings.ToUpper(text)
		}
		return text, nil
	case 'f', 'F', 'g', 'G':
		number, ok := numericFloat(value)
		if !ok {
			return "", newException("TypeError", "float format requires a real number")
		}
		precision := 6
		if dot := strings.IndexByte(specification, '.'); dot >= 0 {
			if parsed, err := strconv.Atoi(strings.TrimLeft(specification[dot+1:], "*")); err == nil {
				precision = parsed
			}
		}
		verb := conversion
		return strconv.FormatFloat(number, verb, precision, 64), nil
	case 'c':
		integer, ok := integerOperand(value)
		if ok && integer.IsInt64() {
			return string(rune(integer.Int64())), nil
		}
		if text, ok := value.(*stringValue); ok && len([]rune(text.value)) == 1 {
			return text.value, nil
		}
		return "", newException("TypeError", "%c requires int or char")
	default:
		return "", newException("ValueError", fmt.Sprintf("unsupported format character %q", conversion))
	}
}

func applyStringWidth(value, specification string) string {
	widthText := strings.Trim(specification, "#0- +")
	width, err := strconv.Atoi(widthText)
	if err != nil || width <= len(value) {
		return value
	}
	padding := strings.Repeat(" ", width-len(value))
	if strings.Contains(specification, "-") {
		return value + padding
	}
	return padding + value
}
