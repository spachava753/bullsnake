package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// compileMatchStatement keeps one subject across failed cases and consumes it
// before the selected suite, with guards participating in ordinary case failure.
func (compiler *compilerState) compileMatchStatement(statement *compilerast.MatchStmt) error {
	baseDepth := compiler.stackDepth
	if err := compiler.compileExpr(statement.Subject); err != nil {
		return err
	}
	end := compiler.newLabel()
	for _, matchCase := range statement.Cases {
		canFail := patternCanFail(matchCase.Pattern) || matchCase.Guard != nil
		var failure *jumpLabel
		if canFail {
			failure = compiler.newLabel()
		}
		if err := compiler.emit(bytecode.Copy, 1, matchCase.Range); err != nil {
			return err
		}
		if err := compiler.compilePattern(matchCase.Pattern, failure, baseDepth+1); err != nil {
			return err
		}
		if matchCase.Guard != nil {
			if err := compiler.compileExpr(matchCase.Guard); err != nil {
				return err
			}
			if err := compiler.emitPatternCheck(failure, baseDepth+1, matchCase.Guard.Span()); err != nil {
				return err
			}
		}
		if err := compiler.emit(bytecode.PopTop, 0, matchCase.Range); err != nil {
			return err
		}
		if err := compiler.compileStatements(matchCase.Body); err != nil {
			return err
		}
		if compiler.reachable {
			if err := compiler.emitJump(bytecode.Jump, end, matchCase.Range); err != nil {
				return err
			}
		}
		if !canFail {
			return compiler.markLabel(end, statement.Span())
		}
		if err := compiler.markLabel(failure, matchCase.Range); err != nil {
			return err
		}
	}
	if compiler.reachable {
		if err := compiler.emit(bytecode.PopTop, 0, statement.Span()); err != nil {
			return err
		}
	}
	return compiler.markLabel(end, statement.Span())
}

// compilePattern consumes one subject, binds captures only through normal name
// stores, and routes every structural miss to a shared restored stack depth.
func (compiler *compilerState) compilePattern(
	pattern compilerast.Pattern,
	failure *jumpLabel,
	failureDepth int,
) error {
	switch pattern := pattern.(type) {
	case *compilerast.ValuePattern:
		if err := compiler.compileExpr(pattern.Value); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.CompareOp, bytecode.CompareEqual, pattern.Span()); err != nil {
			return err
		}
		return compiler.emitPatternCheck(failure, failureDepth, pattern.Span())
	case *compilerast.CapturePattern:
		return compiler.emitNameStore(pattern.Name, pattern.Span())
	case *compilerast.WildcardPattern:
		return compiler.emit(bytecode.PopTop, 0, pattern.Span())
	case *compilerast.StarPattern:
		if pattern.Name == "" {
			return compiler.emit(bytecode.PopTop, 0, pattern.Span())
		}
		return compiler.emitNameStore(pattern.Name, pattern.Span())
	case *compilerast.SequencePattern:
		return compiler.compileSequencePattern(pattern, failure, failureDepth)
	case *compilerast.MappingPattern:
		return compiler.compileMappingPattern(pattern, failure, failureDepth)
	case *compilerast.ClassPattern:
		return compiler.compileClassPattern(pattern, failure, failureDepth)
	case *compilerast.OrPattern:
		return compiler.compileOrPattern(pattern, failure, failureDepth)
	case *compilerast.AsPattern:
		if pattern.Pattern != nil {
			if err := compiler.emit(bytecode.Copy, 1, pattern.Span()); err != nil {
				return err
			}
			if err := compiler.compilePattern(pattern.Pattern, failure, failureDepth); err != nil {
				return err
			}
		}
		if pattern.Name == "" {
			return compiler.emit(bytecode.PopTop, 0, pattern.Span())
		}
		return compiler.emitNameStore(pattern.Name, pattern.Span())
	default:
		return compiler.unsupported(pattern)
	}
}

func (compiler *compilerState) compileSequencePattern(
	pattern *compilerast.SequencePattern,
	failure *jumpLabel,
	failureDepth int,
) error {
	starIndex := -1
	for index, child := range pattern.Elements {
		if _, starred := child.(*compilerast.StarPattern); starred {
			starIndex = index
		}
	}
	operand, ok := bytecode.PackMatchSequence(len(pattern.Elements), starIndex)
	if !ok {
		return compiler.error(pattern.Span(), "sequence pattern is too large")
	}
	if err := compiler.emit(bytecode.MatchSequence, operand, pattern.Span()); err != nil {
		return err
	}
	return compiler.compilePatternResult(pattern.Elements, failure, failureDepth, pattern.Span())
}

// compileMappingPattern evaluates requested keys and unpacks matched values
// plus an optional remainder into their nested patterns.
func (compiler *compilerState) compileMappingPattern(
	pattern *compilerast.MappingPattern,
	failure *jumpLabel,
	failureDepth int,
) error {
	for _, key := range pattern.Keys {
		if err := compiler.compileExpr(key); err != nil {
			return err
		}
	}
	if err := compiler.emit(bytecode.BuildTuple, uint32(len(pattern.Keys)), pattern.Span()); err != nil {
		return err
	}
	rest := uint32(0)
	patterns := append([]compilerast.Pattern(nil), pattern.Patterns...)
	if pattern.Rest != "" {
		rest = 1
		patterns = append(patterns, &compilerast.CapturePattern{
			Range: pattern.Range,
			Name:  pattern.Rest,
		})
	}
	if err := compiler.emit(bytecode.MatchMapping, rest, pattern.Span()); err != nil {
		return err
	}
	return compiler.compilePatternResult(patterns, failure, failureDepth, pattern.Span())
}

// compileClassPattern evaluates the class and keyword names before unpacking
// positional and named attributes into their nested patterns.
func (compiler *compilerState) compileClassPattern(
	pattern *compilerast.ClassPattern,
	failure *jumpLabel,
	failureDepth int,
) error {
	if err := compiler.compileExpr(pattern.Class); err != nil {
		return err
	}
	patterns := append([]compilerast.Pattern(nil), pattern.Positional...)
	for _, keyword := range pattern.Keywords {
		if err := compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(bytecode.TextString(compiler.mangleName(keyword.Name))),
			keyword.Range,
		); err != nil {
			return err
		}
		patterns = append(patterns, keyword.Pattern)
	}
	if err := compiler.emit(bytecode.BuildTuple, uint32(len(pattern.Keywords)), pattern.Span()); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.MatchClass, uint32(len(pattern.Positional)), pattern.Span()); err != nil {
		return err
	}
	return compiler.compilePatternResult(patterns, failure, failureDepth, pattern.Span())
}

// compilePatternResult distinguishes the internal miss marker from extracted
// values, restores failure state, and recursively consumes successful fields.
func (compiler *compilerState) compilePatternResult(
	patterns []compilerast.Pattern,
	failure *jumpLabel,
	failureDepth int,
	span lexer.Span,
) error {
	if err := compiler.emit(bytecode.Copy, 1, span); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.None()),
		span,
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.CompareOp, bytecode.CompareIsNot, span); err != nil {
		return err
	}
	if err := compiler.emitPatternCheck(failure, failureDepth, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.UnpackSequence, uint32(len(patterns)), span); err != nil {
		return err
	}
	for _, child := range patterns {
		if err := compiler.compilePattern(child, failure, failureDepth); err != nil {
			return err
		}
	}
	return nil
}

// compileOrPattern retries each alternative against a retained subject and
// removes that subject after the first successful alternative.
func (compiler *compilerState) compileOrPattern(
	pattern *compilerast.OrPattern,
	failure *jumpLabel,
	failureDepth int,
) error {
	success := compiler.newLabel()
	for index, alternative := range pattern.Patterns {
		alternativeFailure := compiler.newLabel()
		if err := compiler.emit(bytecode.Copy, 1, alternative.Span()); err != nil {
			return err
		}
		if err := compiler.compilePattern(
			alternative,
			alternativeFailure,
			failureDepth+1,
		); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.PopTop, 0, alternative.Span()); err != nil {
			return err
		}
		if err := compiler.emitJump(bytecode.Jump, success, alternative.Span()); err != nil {
			return err
		}
		if err := compiler.markLabel(alternativeFailure, alternative.Span()); err != nil {
			return err
		}
		if index == len(pattern.Patterns)-1 {
			if err := compiler.emitPatternFailure(failure, failureDepth, alternative.Span()); err != nil {
				return err
			}
		}
	}
	return compiler.markLabel(success, pattern.Span())
}

func (compiler *compilerState) emitPatternCheck(
	failure *jumpLabel,
	failureDepth int,
	span lexer.Span,
) error {
	matched := compiler.newLabel()
	if err := compiler.emitJump(bytecode.PopJumpIfTrue, matched, span); err != nil {
		return err
	}
	if err := compiler.emitPatternFailure(failure, failureDepth, span); err != nil {
		return err
	}
	return compiler.markLabel(matched, span)
}

func (compiler *compilerState) emitPatternFailure(
	failure *jumpLabel,
	failureDepth int,
	span lexer.Span,
) error {
	if failure == nil {
		return compiler.error(span, "irrefutable pattern emitted a failure path")
	}
	for compiler.stackDepth > failureDepth {
		if err := compiler.emit(bytecode.PopTop, 0, span); err != nil {
			return err
		}
	}
	if compiler.stackDepth != failureDepth {
		return compiler.error(span, "pattern failure stack depth is inconsistent")
	}
	return compiler.emitJump(bytecode.Jump, failure, span)
}

func patternCanFail(pattern compilerast.Pattern) bool {
	switch pattern := pattern.(type) {
	case *compilerast.CapturePattern, *compilerast.WildcardPattern, *compilerast.StarPattern:
		return false
	case *compilerast.AsPattern:
		return pattern.Pattern != nil && patternCanFail(pattern.Pattern)
	default:
		return true
	}
}
