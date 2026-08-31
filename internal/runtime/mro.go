package runtime

import "strings"

// calculateMRO merges each direct base's linearization with the declared base
// order, preserving local precedence and monotonicity.
func calculateMRO(class *typeValue, bases []*typeValue) ([]*typeValue, *Exception) {
	seen := make(map[*typeValue]struct{}, len(bases))
	for _, base := range bases {
		if _, duplicate := seen[base]; duplicate {
			return nil, newException("TypeError", "duplicate base class "+base.name)
		}
		seen[base] = struct{}{}
	}

	sequences := make([][]*typeValue, 0, len(bases)+1)
	for _, base := range bases {
		sequences = append(sequences, base.mro)
	}
	sequences = append(sequences, bases)
	positions := make([]int, len(sequences))
	result := []*typeValue{class}
	for {
		candidate, complete := nextMROCandidate(sequences, positions)
		if complete {
			return result, nil
		}
		if candidate == nil {
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

func nextMROCandidate(sequences [][]*typeValue, positions []int) (*typeValue, bool) {
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
	return nil, complete
}

func mroTailContains(
	sequences [][]*typeValue,
	positions []int,
	candidate *typeValue,
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

func inconsistentMROError(sequences [][]*typeValue, positions []int) *Exception {
	names := make([]string, 0, len(sequences))
	seen := make(map[*typeValue]struct{}, len(sequences))
	for index, sequence := range sequences {
		if positions[index] >= len(sequence) {
			continue
		}
		head := sequence[positions[index]]
		if _, duplicate := seen[head]; duplicate {
			continue
		}
		seen[head] = struct{}{}
		names = append(names, head.name)
	}
	return newException(
		"TypeError",
		"Cannot create a consistent method resolution order (MRO) for bases "+
			strings.Join(names, ", "),
	)
}
