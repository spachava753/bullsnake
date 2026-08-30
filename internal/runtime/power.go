package runtime

import (
	"math"
	"math/big"
)

const maxIntegerPowerBits = 1 << 20

func integerPowerLimitException() *Exception {
	return newException(
		"OverflowError",
		"integer power result exceeds 1048576-bit limit",
	)
}

// integerPower implements int and bool exponentiation. Nonnegative exponents
// stay exact, while negative exponents follow CPython's real-float path.
func integerPower(base, exponent *big.Int) (Value, *Exception) {
	if exponent.Sign() < 0 {
		return negativeIntegerPower(base, exponent)
	}
	if exponent.Sign() == 0 {
		return integerFromInt64(1), nil
	}

	var absoluteBase big.Int
	absoluteBase.Abs(base)
	var one big.Int
	one.SetInt64(1)
	switch absoluteBase.Cmp(&one) {
	case -1:
		return &intValue{}, nil
	case 0:
		if base.Sign() < 0 && exponent.Bit(0) == 1 {
			return integerFromInt64(-1), nil
		}
		return integerFromInt64(1), nil
	}
	return exactIntegerPower(base, exponent)
}

// exactIntegerPower uses exponentiation by squaring and checks every retained
// intermediate so one operation cannot construct an unbounded Go big integer.
func exactIntegerPower(base, exponent *big.Int) (Value, *Exception) {
	if base.BitLen() > maxIntegerPowerBits {
		return nil, integerPowerLimitException()
	}
	var remaining big.Int
	remaining.Set(exponent)
	var result big.Int
	result.SetInt64(1)
	var factor big.Int
	factor.Set(base)
	for remaining.Sign() > 0 {
		if remaining.Bit(0) == 1 {
			result.Mul(&result, &factor)
			if result.BitLen() > maxIntegerPowerBits {
				return nil, integerPowerLimitException()
			}
		}
		remaining.Rsh(&remaining, 1)
		if remaining.Sign() > 0 {
			factor.Mul(&factor, &factor)
			if factor.BitLen() > maxIntegerPowerBits {
				return nil, integerPowerLimitException()
			}
		}
	}
	return &intValue{value: result}, nil
}

func negativeIntegerPower(base, exponent *big.Int) (Value, *Exception) {
	baseFloat, _ := base.Float64()
	if math.IsInf(baseFloat, 0) {
		return nil, newException("OverflowError", "int too large to convert to float")
	}
	exponentFloat, _ := exponent.Float64()
	if math.IsInf(exponentFloat, 0) {
		return nil, newException("OverflowError", "int too large to convert to float")
	}
	if base.Sign() == 0 {
		return nil, newException("ZeroDivisionError", "zero to a negative power")
	}
	result := math.Pow(baseFloat, exponentFloat)
	if math.IsInf(result, 0) {
		return nil, newException("OverflowError", "math range error")
	}
	return &floatValue{value: result}, nil
}

func integerFromInt64(value int64) *intValue {
	var integer big.Int
	integer.SetInt64(value)
	return &intValue{value: integer}
}
