package runtime

import (
	"strconv"
	"strings"
)

type stringReplaceMethod struct {
	value *stringValue
}

func (*stringReplaceMethod) TypeName() string { return "builtin_function_or_method" }
func (*stringReplaceMethod) Repr() string {
	return "<built-in method replace of str object>"
}
func (*stringReplaceMethod) isValue() {}

// executeStringReplaceCall binds the positional strings and optional count,
// then preserves the receiver when replacement changes no bytes.
func executeStringReplaceCall(
	caller *frame,
	instruction int,
	base int,
	method *stringReplaceMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	old, replacement, count, exception := bindStringReplaceArguments(arguments, keywords)
	if exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}
	limit, exception := stringReplaceLimit(count)
	if exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}

	result := method.value.value
	if limit != 0 {
		if old.value == "" {
			result = replaceEmptyString(result, replacement.value, limit)
		} else {
			result = strings.Replace(result, old.value, replacement.value, limit)
		}
	}
	discardCallSegment(caller, base)
	if result == method.value.value {
		return pushOutcome(caller, instruction, method.value)
	}
	return pushOutcome(caller, instruction, &stringValue{value: result})
}

// bindStringReplaceArguments accepts two positional-only strings and count as
// either a third positional value or its CPython 3.14 keyword form.
func bindStringReplaceArguments(
	arguments []Value,
	keywords *dictValue,
) (*stringValue, *stringValue, Value, *Exception) {
	if len(arguments) < 2 {
		return nil, nil, nil, newException(
			"TypeError",
			"replace expected at least 2 arguments, got "+strconv.Itoa(len(arguments)),
		)
	}
	if len(arguments) > 3 {
		return nil, nil, nil, newException(
			"TypeError",
			"replace expected at most 3 arguments, got "+strconv.Itoa(len(arguments)),
		)
	}
	old, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, nil, nil, newException(
			"TypeError",
			"replace() argument 1 must be str, not "+arguments[0].TypeName(),
		)
	}
	replacement, ok := arguments[1].(*stringValue)
	if !ok {
		return nil, nil, nil, newException(
			"TypeError",
			"replace() argument 2 must be str, not "+arguments[1].TypeName(),
		)
	}

	count := Value(integerFromInt64(-1))
	assigned := false
	if len(arguments) == 3 {
		count = arguments[2]
		assigned = true
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if !ok {
				return nil, nil, nil, newException("TypeError", "keywords must be strings")
			}
			if name.value != "count" {
				return nil, nil, nil, newException(
					"TypeError",
					"'"+name.value+"' is an invalid keyword argument for replace()",
				)
			}
			if assigned {
				return nil, nil, nil, newException(
					"TypeError",
					"replace() got multiple values for argument 'count'",
				)
			}
			count = entry.value
			assigned = true
		}
	}
	return old, replacement, count, nil
}

func stringReplaceLimit(value Value) (int, *Exception) {
	integer, ok := integerOperand(value)
	if !ok {
		return 0, newException(
			"TypeError",
			"'"+value.TypeName()+"' object cannot be interpreted as an integer",
		)
	}
	if !integer.IsInt64() {
		return 0, newException(
			"OverflowError",
			"Python int too large to convert to C ssize_t",
		)
	}
	limit := integer.Int64()
	if int64(int(limit)) != limit {
		return 0, newException(
			"OverflowError",
			"Python int too large to convert to C ssize_t",
		)
	}
	if limit < 0 {
		return -1, nil
	}
	return int(limit), nil
}

// replaceEmptyString inserts replacements at the first requested code-point
// boundaries and copies the untouched suffix after the final insertion.
func replaceEmptyString(value, replacement string, limit int) string {
	offsets := stringCodepointOffsets(value)
	boundaries := len(offsets)
	if limit < 0 || limit > boundaries {
		limit = boundaries
	}
	var builder strings.Builder
	for boundary := 0; boundary < limit; boundary++ {
		builder.WriteString(replacement)
		if boundary+1 < len(offsets) {
			builder.WriteString(value[offsets[boundary]:offsets[boundary+1]])
		}
	}
	if limit < len(offsets) {
		builder.WriteString(value[offsets[limit]:])
	}
	return builder.String()
}
