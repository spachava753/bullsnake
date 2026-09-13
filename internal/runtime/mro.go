package runtime

import "strings"

// calculateMRO merges each direct base's linearization with the declared base
// order, preserving local precedence and monotonicity.
func calculateMRO[T interface {
	Value
	comparable
}](class T, bases []T, baseOrder func(T) []T) ([]T, *Exception) {
	seen := make(map[T]struct{}, len(bases))
	for _, base := range bases {
		if _, duplicate := seen[base]; duplicate {
			return nil, newException("TypeError", "duplicate base class "+allocationClassName(base))
		}
		seen[base] = struct{}{}
	}

	sequences := make([][]T, 0, len(bases)+1)
	for _, base := range bases {
		sequences = append(sequences, baseOrder(base))
	}
	sequences = append(sequences, bases)
	positions := make([]int, len(sequences))
	result := []T{class}
	var zero T
	for {
		candidate, complete := nextMROCandidate(sequences, positions)
		if complete {
			return result, nil
		}
		if candidate == zero {
			return nil, inconsistentMROError(sequences, positions)
		}
		result = append(result, candidate)
		for index, sequence := range sequences {
			if positions[index] < len(sequence) && sequence[positions[index]] == candidate {
				positions[index]++
			}
		}
	}
}

func nextMROCandidate[T comparable](sequences [][]T, positions []int) (T, bool) {
	complete := true
	for index, sequence := range sequences {
		position := positions[index]
		if position >= len(sequence) {
			continue
		}
		complete = false
		candidate := sequence[position]
		if !mroTailContains(sequences, positions, candidate) {
			return candidate, false
		}
	}
	var zero T
	return zero, complete
}

func mroTailContains[T comparable](
	sequences [][]T,
	positions []int,
	candidate T,
) bool {
	for index, sequence := range sequences {
		for position := positions[index] + 1; position < len(sequence); position++ {
			if sequence[position] == candidate {
				return true
			}
		}
	}
	return false
}

func inconsistentMROError[T interface {
	Value
	comparable
}](sequences [][]T, positions []int) *Exception {
	names := make([]string, 0, len(sequences))
	seen := make(map[T]struct{}, len(sequences))
	for index, sequence := range sequences {
		if positions[index] >= len(sequence) {
			continue
		}
		head := sequence[positions[index]]
		if _, duplicate := seen[head]; duplicate {
			continue
		}
		seen[head] = struct{}{}
		names = append(names, allocationClassName(head))
	}
	return newException(
		"TypeError",
		"Cannot create a consistent method resolution order (MRO) for bases "+
			strings.Join(names, ", "),
	)
}
