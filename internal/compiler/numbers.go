package compiler

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

// parseNumberLiteral canonicalizes integer bases or converts float and
// imaginary spellings to their binary64 values.
func parseNumberLiteral(text string) (bytecode.Constant, error) {
	clean := strings.ReplaceAll(text, "_", "")
	if strings.HasSuffix(clean, "j") || strings.HasSuffix(clean, "J") {
		value, err := parseFloatLiteral(clean[:len(clean)-1])
		if err != nil {
			return bytecode.Constant{}, err
		}
		return bytecode.Imaginary(value), nil
	}
	prefixedInteger := len(clean) >= 2 && clean[0] == '0' &&
		strings.ContainsRune("bBoOxX", rune(clean[1]))
	if !prefixedInteger && strings.ContainsAny(clean, ".eE") {
		value, err := parseFloatLiteral(clean)
		if err != nil {
			return bytecode.Constant{}, err
		}
		return bytecode.Float(value), nil
	}
	value, ok := new(big.Int).SetString(clean, 0)
	if !ok {
		return bytecode.Constant{}, fmt.Errorf("invalid integer literal %q", text)
	}
	return bytecode.Integer(value.String()), nil
}

func parseFloatLiteral(text string) (float64, error) {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil && !math.IsInf(value, 0) {
		return 0, fmt.Errorf("invalid floating-point literal %q", text)
	}
	return value, nil
}
