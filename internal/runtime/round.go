package runtime

import (
	"math"
	"math/big"
	"strconv"
)

const (
	maximumFloatRoundDigits = 323
	minimumFloatRoundDigits = -308
)

// executeBuiltinRound binds the two public arguments before handling native
// numbers or dispatching a user __round__ method through the frame loop.
func executeBuiltinRound(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	number, digits, exception := bindRoundArguments(arguments, keywords)
	discardCallSegment(caller, base)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	if result, exception, supported := immediateRound(number, digits); supported {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, instruction, result)
	}

	instance, ok := number.(*instanceValue)
	if !ok {
		return raiseOutcome(newException(
			"TypeError",
			"type "+number.TypeName()+" doesn't define __round__ method",
		)), nil
	}
	method, found := lookupInstanceSpecial(instance, "__round__")
	if !found {
		return raiseOutcome(newException(
			"TypeError",
			"type "+number.TypeName()+" doesn't define __round__ method",
		)), nil
	}
	var methodArguments []Value
	if digits != None {
		methodArguments = []Value{digits}
	}
	return executeFunctionCall(
		caller,
		instruction,
		len(caller.stack),
		method,
		methodArguments,
		nil,
	)
}

// bindRoundArguments applies CPython's positional and named binding for number
// and ndigits without invoking either value.
func bindRoundArguments(
	arguments []Value,
	keywords *dictValue,
) (Value, Value, *Exception) {
	if len(arguments) > 2 {
		return nil, nil, newException(
			"TypeError",
			"round() takes at most 2 arguments ("+
				strconv.Itoa(len(arguments))+" given)",
		)
	}
	values := [2]Value{nil, None}
	assigned := [2]bool{}
	for index, value := range arguments {
		values[index] = value
		assigned[index] = true
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			nameValue, ok := entry.key.(*stringValue)
			if !ok {
				return nil, nil, newException("TypeError", "keywords must be strings")
			}
			name := nameValue.value
			index := -1
			switch name {
			case "number":
				index = 0
			case "ndigits":
				index = 1
			default:
				return nil, nil, newException(
					"TypeError",
					"'"+name+"' is an invalid keyword argument for round()",
				)
			}
			if assigned[index] {
				return nil, nil, newException(
					"TypeError",
					"round() got multiple values for argument '"+name+"'",
				)
			}
			values[index] = entry.value
			assigned[index] = true
		}
	}
	if !assigned[0] {
		return nil, nil, newException(
			"TypeError",
			"round() missing required argument 'number' (pos 1)",
		)
	}
	return values[0], values[1], nil
}

// immediateRound handles the current native integer and float methods and
// reports every other value for user special-method dispatch.
func immediateRound(number, digits Value) (Value, *Exception, bool) {
	integer, integerNumber := integerOperand(number)
	if integerNumber {
		if digits == None {
			if exact, ok := number.(*intValue); ok {
				return exact, nil, true
			}
			return &intValue{value: integer}, nil, true
		}
		digitCount, exception := roundDigitCount(digits)
		if exception != nil {
			return nil, exception, true
		}
		result := roundInteger(integer, digitCount)
		return &intValue{value: result}, nil, true
	}

	floating, floatingNumber := number.(*floatValue)
	if !floatingNumber {
		return nil, nil, false
	}
	if digits == None {
		result, exception := roundFloatToInteger(floating.value)
		return result, exception, true
	}
	digitCount, exception := roundDigitCount(digits)
	if exception != nil {
		return nil, exception, true
	}
	return roundFloat(floating.value, digitCount), nil, true
}

func roundDigitCount(value Value) (*big.Int, *Exception) {
	integer, ok := integerOperand(value)
	if !ok {
		return nil, newException(
			"TypeError",
			"'"+value.TypeName()+"' object cannot be interpreted as an integer",
		)
	}
	return &integer, nil
}

func roundInteger(value big.Int, digits *big.Int) big.Int {
	if digits.Sign() >= 0 || value.Sign() == 0 {
		return *new(big.Int).Set(&value)
	}
	exponent := new(big.Int).Neg(digits)
	absolute := new(big.Int).Abs(&value)
	decimalDigits := int64(len(absolute.Text(10)))
	if !exponent.IsInt64() || exponent.Int64() > decimalDigits {
		return *new(big.Int)
	}
	power := decimalPower(int(exponent.Int64()))
	quotient := nearestEvenQuotient(&value, power)
	return *new(big.Int).Mul(&quotient, power)
}

func roundFloatToInteger(value float64) (Value, *Exception) {
	switch {
	case math.IsInf(value, 0):
		return nil, newException(
			"OverflowError",
			"cannot convert float infinity to integer",
		)
	case math.IsNaN(value):
		return nil, newException(
			"ValueError",
			"cannot convert float NaN to integer",
		)
	}
	rounded := math.RoundToEven(value)
	integer, _ := new(big.Float).SetFloat64(rounded).Int(nil)
	return &intValue{value: *integer}, nil
}

// roundFloat quantizes the exact binary value to a decimal power and converts
// the result back to binary64 with ties directed to the even neighbor.
func roundFloat(value float64, digits *big.Int) Value {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return &floatValue{value: value}
	}
	maximum := big.NewInt(maximumFloatRoundDigits)
	minimum := big.NewInt(minimumFloatRoundDigits)
	if digits.Cmp(maximum) > 0 {
		return &floatValue{value: value}
	}
	if digits.Cmp(minimum) < 0 {
		return &floatValue{value: math.Copysign(0, value)}
	}

	digitCount := int(digits.Int64())
	exponent := digitCount
	if exponent < 0 {
		exponent = -exponent
	}
	power := decimalPower(exponent)
	valueRatio := new(big.Rat).SetFloat64(value)
	var roundedRatio big.Rat
	if digitCount >= 0 {
		numerator := new(big.Int).Mul(valueRatio.Num(), power)
		quotient := nearestEvenQuotient(numerator, valueRatio.Denom())
		roundedRatio.SetFrac(&quotient, power)
	} else {
		denominator := new(big.Int).Mul(valueRatio.Denom(), power)
		quotient := nearestEvenQuotient(valueRatio.Num(), denominator)
		integer := new(big.Int).Mul(&quotient, power)
		roundedRatio.SetInt(integer)
	}
	if roundedRatio.Sign() == 0 {
		return &floatValue{value: math.Copysign(0, value)}
	}
	floating := new(big.Float).SetPrec(53).SetMode(big.ToNearestEven)
	floating.SetRat(&roundedRatio)
	result, _ := floating.Float64()
	return &floatValue{value: result}
}

func decimalPower(exponent int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil)
}

// nearestEvenQuotient rounds one rational value to an integer and chooses the
// even neighbor at an exact half.
func nearestEvenQuotient(numerator, denominator *big.Int) big.Int {
	sign := numerator.Sign()
	absolute := new(big.Int).Abs(numerator)
	var quotient, remainder big.Int
	quotient.QuoRem(absolute, denominator, &remainder)
	twiceRemainder := new(big.Int).Lsh(&remainder, 1)
	comparison := twiceRemainder.Cmp(denominator)
	if comparison > 0 || comparison == 0 && quotient.Bit(0) != 0 {
		quotient.Add(&quotient, big.NewInt(1))
	}
	if sign < 0 {
		quotient.Neg(&quotient)
	}
	return quotient
}
