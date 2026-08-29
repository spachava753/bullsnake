package resolver

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type patternCapture struct {
	name string
	span lexer.Span
}

type patternInfo struct {
	captures          []patternCapture
	names             nameSet
	irrefutable       bool
	irrefutableReason string
}

// collectMatch resolves the subject, validates and binds each pattern, then
// visits its guard and body in source order.
func (state *resolver) collectMatch(statement *compilerast.MatchStmt) error {
	if err := state.collectExpr(statement.Subject); err != nil {
		return err
	}
	for _, matchCase := range statement.Cases {
		info, err := state.inspectPattern(matchCase.Pattern)
		if err != nil {
			return err
		}
		for _, capture := range info.captures {
			if _, err := state.bind(capture.name, Assigned, capture.span); err != nil {
				return err
			}
		}
		if err := state.collectExpr(matchCase.Guard); err != nil {
			return err
		}
		if err := state.collectStatements(matchCase.Body); err != nil {
			return err
		}
	}
	return nil
}

// inspectPattern collects one pattern's captures and irrefutability while
// resolving embedded value expressions without binding names yet.
func (state *resolver) inspectPattern(pattern compilerast.Pattern) (patternInfo, error) {
	switch pattern := pattern.(type) {
	case *compilerast.ValuePattern:
		return patternInfo{}, state.collectExpr(pattern.Value)
	case *compilerast.CapturePattern:
		return state.newPatternCapture(pattern.Name, pattern.Span(), true, "capture"), nil
	case *compilerast.WildcardPattern:
		return patternInfo{irrefutable: true, irrefutableReason: "wildcard"}, nil
	case *compilerast.StarPattern:
		if pattern.Name == "" {
			return patternInfo{}, nil
		}
		return state.newPatternCapture(pattern.Name, pattern.Span(), false, ""), nil
	case *compilerast.SequencePattern:
		return state.inspectPatternSequence(pattern.Elements)
	case *compilerast.MappingPattern:
		for _, key := range pattern.Keys {
			if err := state.collectExpr(key); err != nil {
				return patternInfo{}, err
			}
		}
		info, err := state.inspectPatternSequence(pattern.Patterns)
		if err != nil {
			return patternInfo{}, err
		}
		if pattern.Rest != "" {
			if err := state.addPatternCapture(&info, pattern.Rest, pattern.Span()); err != nil {
				return patternInfo{}, err
			}
		}
		info.irrefutable = false
		info.irrefutableReason = ""
		return info, nil
	case *compilerast.ClassPattern:
		if err := state.collectExpr(pattern.Class); err != nil {
			return patternInfo{}, err
		}
		patterns := append([]compilerast.Pattern(nil), pattern.Positional...)
		for _, keyword := range pattern.Keywords {
			patterns = append(patterns, keyword.Pattern)
		}
		info, err := state.inspectPatternSequence(patterns)
		info.irrefutable = false
		info.irrefutableReason = ""
		return info, err
	case *compilerast.OrPattern:
		return state.inspectOrPattern(pattern)
	case *compilerast.AsPattern:
		info := patternInfo{names: make(nameSet)}
		var err error
		if pattern.Pattern != nil {
			info, err = state.inspectPattern(pattern.Pattern)
			if err != nil {
				return patternInfo{}, err
			}
		} else {
			info.irrefutable = true
			info.irrefutableReason = "capture"
		}
		if pattern.Name != "" {
			if err := state.addPatternCapture(&info, pattern.Name, pattern.Span()); err != nil {
				return patternInfo{}, err
			}
		}
		return info, nil
	default:
		return patternInfo{}, unsupportedNode(pattern)
	}
}

func (state *resolver) inspectPatternSequence(patterns []compilerast.Pattern) (patternInfo, error) {
	info := patternInfo{names: make(nameSet)}
	for _, pattern := range patterns {
		child, err := state.inspectPattern(pattern)
		if err != nil {
			return patternInfo{}, err
		}
		for _, capture := range child.captures {
			if err := state.addPatternCapture(&info, capture.name, capture.span); err != nil {
				return patternInfo{}, err
			}
		}
	}
	return info, nil
}

// inspectOrPattern requires every alternative to bind the same names and keeps
// irrefutable alternatives in the final position.
func (state *resolver) inspectOrPattern(pattern *compilerast.OrPattern) (patternInfo, error) {
	var first patternInfo
	for index, alternative := range pattern.Patterns {
		info, err := state.inspectPattern(alternative)
		if err != nil {
			return patternInfo{}, err
		}
		if index == 0 {
			first = info
		} else if !sameNames(first.names, info.names) {
			return patternInfo{}, state.syntaxError(
				alternative.Span(), "alternative patterns bind different names",
			)
		}
		if info.irrefutable && index != len(pattern.Patterns)-1 {
			if info.irrefutableReason == "wildcard" {
				return patternInfo{}, state.syntaxError(
					alternative.Span(), "wildcard makes remaining patterns unreachable",
				)
			}
			return patternInfo{}, state.syntaxError(
				alternative.Span(), "name capture makes remaining patterns unreachable",
			)
		}
		if index == len(pattern.Patterns)-1 {
			first.irrefutable = info.irrefutable
			first.irrefutableReason = info.irrefutableReason
		}
	}
	return first, nil
}

func (state *resolver) newPatternCapture(name string, span lexer.Span, irrefutable bool, reason string) patternInfo {
	name = manglePrivate(state.current.PrivateName, name)
	return patternInfo{
		captures:          []patternCapture{{name: name, span: span}},
		names:             nameSet{name: {}},
		irrefutable:       irrefutable,
		irrefutableReason: reason,
	}
}

func (state *resolver) addPatternCapture(info *patternInfo, name string, span lexer.Span) error {
	if info.names == nil {
		info.names = make(nameSet)
	}
	name = manglePrivate(state.current.PrivateName, name)
	if hasName(info.names, name) {
		return state.syntaxError(span, "multiple assignments to name %q in pattern", name)
	}
	info.names[name] = struct{}{}
	info.captures = append(info.captures, patternCapture{name: name, span: span})
	return nil
}

func sameNames(left, right nameSet) bool {
	if len(left) != len(right) {
		return false
	}
	for name := range left {
		if !hasName(right, name) {
			return false
		}
	}
	return true
}
