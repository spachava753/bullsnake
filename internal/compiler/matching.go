package compiler

import (
	"fmt"
	"strconv"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type matchCapture struct {
	name string
	span lexer.Span
}

// compileMatchStatement evaluates one subject and dispatches to the first
// basic pattern whose optional guard succeeds.
func (compiler *compilerState) compileMatchStatement(
	statement *compilerast.MatchStmt,
) error {
	if len(statement.Cases) == 0 {
		return compiler.error(statement.Span(), "match statement has no cases")
	}
	if err := compiler.compileExpr(statement.Subject); err != nil {
		return err
	}

	end := compiler.newLabel()
	for _, matchCase := range statement.Cases {
		matched := compiler.newLabel()
		failed := compiler.newLabel()
		captures, err := compiler.patternCaptures(matchCase.Pattern)
		if err != nil {
			return err
		}
		extracting := patternNeedsExtraction(matchCase.Pattern)
		var captureTemps map[string]uint32
		if extracting {
			captureTemps = compiler.allocateMatchTemps(captures)
			failureDepth := compiler.stackDepth
			if err := compiler.emit(bytecode.Copy, 1, matchCase.Pattern.Span()); err != nil {
				return err
			}
			if err := compiler.compileExtractingPattern(
				matchCase.Pattern,
				failed,
				failureDepth,
				captureTemps,
			); err != nil {
				return err
			}
			if err := compiler.emitJump(bytecode.Jump, matched, matchCase.Pattern.Span()); err != nil {
				return err
			}
		} else if err := compiler.compileBasicPattern(
			matchCase.Pattern,
			matched,
			failed,
		); err != nil {
			return err
		}
		if err := compiler.markLabel(matched, matchCase.Pattern.Span()); err != nil {
			return err
		}
		for _, capture := range captures {
			if extracting {
				if err := compiler.emit(
					bytecode.LoadFast,
					captureTemps[capture.name],
					capture.span,
				); err != nil {
					return err
				}
			} else if err := compiler.emit(bytecode.Copy, 1, capture.span); err != nil {
				return err
			}
			if err := compiler.emitNameStore(capture.name, capture.span); err != nil {
				return err
			}
		}
		if matchCase.Guard != nil {
			if err := compiler.compileExpr(matchCase.Guard); err != nil {
				return err
			}
			if err := compiler.emitJump(
				bytecode.PopJumpIfFalse,
				failed,
				matchCase.Guard.Span(),
			); err != nil {
				return err
			}
		}
		if err := compiler.emit(bytecode.PopTop, 0, matchCase.Pattern.Span()); err != nil {
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
		if err := compiler.markLabel(failed, matchCase.Pattern.Span()); err != nil {
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

// compileBasicPattern tests one pattern while retaining the match subject on
// the operand stack along both success and failure paths.
func (compiler *compilerState) compileBasicPattern(
	pattern compilerast.Pattern,
	matched *jumpLabel,
	failed *jumpLabel,
) error {
	switch pattern := pattern.(type) {
	case *compilerast.ValuePattern:
		if err := compiler.emit(bytecode.Copy, 1, pattern.Span()); err != nil {
			return err
		}
		if err := compiler.compileExpr(pattern.Value); err != nil {
			return err
		}
		comparison := uint32(bytecode.CompareEqual)
		switch pattern.Value.(type) {
		case *compilerast.NoneLiteral, *compilerast.BooleanLiteral:
			comparison = bytecode.CompareIs
		}
		if err := compiler.emit(bytecode.CompareOp, comparison, pattern.Span()); err != nil {
			return err
		}
		if err := compiler.emitJump(bytecode.PopJumpIfTrue, matched, pattern.Span()); err != nil {
			return err
		}
		return compiler.emitJump(bytecode.Jump, failed, pattern.Span())
	case *compilerast.CapturePattern, *compilerast.WildcardPattern:
		return compiler.emitJump(bytecode.Jump, matched, pattern.Span())
	case *compilerast.AsPattern:
		if pattern.Pattern == nil {
			return compiler.emitJump(bytecode.Jump, matched, pattern.Span())
		}
		return compiler.compileBasicPattern(pattern.Pattern, matched, failed)
	case *compilerast.OrPattern:
		if len(pattern.Patterns) == 0 {
			return compiler.error(pattern.Span(), "OR pattern has no alternatives")
		}
		for _, alternative := range pattern.Patterns[:len(pattern.Patterns)-1] {
			next := compiler.newLabel()
			if err := compiler.compileBasicPattern(alternative, matched, next); err != nil {
				return err
			}
			if err := compiler.markLabel(next, alternative.Span()); err != nil {
				return err
			}
		}
		return compiler.compileBasicPattern(
			pattern.Patterns[len(pattern.Patterns)-1],
			matched,
			failed,
		)
	default:
		return compiler.unsupported(pattern)
	}
}

// compileExtractingPattern consumes one candidate, stores tentative captures
// in hidden locals, and cleans all pattern work before a failure jump.
func (compiler *compilerState) compileExtractingPattern(
	pattern compilerast.Pattern,
	failed *jumpLabel,
	failureDepth int,
	captureTemps map[string]uint32,
) error {
	switch pattern := pattern.(type) {
	case *compilerast.ValuePattern:
		if err := compiler.compileExpr(pattern.Value); err != nil {
			return err
		}
		comparison := uint32(bytecode.CompareEqual)
		switch pattern.Value.(type) {
		case *compilerast.NoneLiteral, *compilerast.BooleanLiteral:
			comparison = bytecode.CompareIs
		}
		if err := compiler.emit(bytecode.CompareOp, comparison, pattern.Span()); err != nil {
			return err
		}
		return compiler.compilePatternCondition(failed, failureDepth, pattern.Span())
	case *compilerast.CapturePattern:
		return compiler.storePatternCapture(pattern.Name, pattern.Span(), captureTemps)
	case *compilerast.WildcardPattern:
		return compiler.emit(bytecode.PopTop, 0, pattern.Span())
	case *compilerast.StarPattern:
		if pattern.Name == "" {
			return compiler.emit(bytecode.PopTop, 0, pattern.Span())
		}
		return compiler.storePatternCapture(pattern.Name, pattern.Span(), captureTemps)
	case *compilerast.AsPattern:
		if pattern.Pattern == nil {
			if pattern.Name == "" {
				return compiler.emit(bytecode.PopTop, 0, pattern.Span())
			}
			return compiler.storePatternCapture(pattern.Name, pattern.Span(), captureTemps)
		}
		if err := compiler.emit(bytecode.Copy, 1, pattern.Span()); err != nil {
			return err
		}
		if err := compiler.compileExtractingPattern(
			pattern.Pattern,
			failed,
			failureDepth,
			captureTemps,
		); err != nil {
			return err
		}
		if pattern.Name == "" {
			return compiler.emit(bytecode.PopTop, 0, pattern.Span())
		}
		return compiler.storePatternCapture(pattern.Name, pattern.Span(), captureTemps)
	case *compilerast.OrPattern:
		return compiler.compileExtractingOrPattern(
			pattern,
			failed,
			failureDepth,
			captureTemps,
		)
	case *compilerast.SequencePattern:
		return compiler.compileSequencePattern(
			pattern,
			failed,
			failureDepth,
			captureTemps,
		)
	default:
		return compiler.unsupported(pattern)
	}
}

// compileExtractingOrPattern retries alternatives with one retained candidate;
// successful alternatives discard it, while failures preserve it for the next.
func (compiler *compilerState) compileExtractingOrPattern(
	pattern *compilerast.OrPattern,
	failed *jumpLabel,
	failureDepth int,
	captureTemps map[string]uint32,
) error {
	if len(pattern.Patterns) == 0 {
		return compiler.error(pattern.Span(), "OR pattern has no alternatives")
	}
	end := compiler.newLabel()
	for _, alternative := range pattern.Patterns[:len(pattern.Patterns)-1] {
		next := compiler.newLabel()
		if err := compiler.emit(bytecode.Copy, 1, alternative.Span()); err != nil {
			return err
		}
		if err := compiler.compileExtractingPattern(
			alternative,
			next,
			compiler.stackDepth-1,
			captureTemps,
		); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.PopTop, 0, alternative.Span()); err != nil {
			return err
		}
		if err := compiler.emitJump(bytecode.Jump, end, alternative.Span()); err != nil {
			return err
		}
		if err := compiler.markLabel(next, alternative.Span()); err != nil {
			return err
		}
	}
	last := pattern.Patterns[len(pattern.Patterns)-1]
	if err := compiler.emit(bytecode.Copy, 1, last.Span()); err != nil {
		return err
	}
	if err := compiler.compileExtractingPattern(
		last,
		failed,
		failureDepth,
		captureTemps,
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.PopTop, 0, last.Span()); err != nil {
		return err
	}
	return compiler.markLabel(end, pattern.Span())
}

// compileSequencePattern verifies tuple/list shape and length before unpacking;
// each nested element consumes one extracted candidate or jumps after cleanup.
func (compiler *compilerState) compileSequencePattern(
	pattern *compilerast.SequencePattern,
	failed *jumpLabel,
	failureDepth int,
	captureTemps map[string]uint32,
) error {
	if err := compiler.emit(bytecode.MatchSequence, 0, pattern.Span()); err != nil {
		return err
	}
	if err := compiler.compilePatternCondition(failed, failureDepth, pattern.Span()); err != nil {
		return err
	}

	star := -1
	for index, element := range pattern.Elements {
		if _, ok := element.(*compilerast.StarPattern); !ok {
			continue
		}
		if star >= 0 {
			return compiler.error(element.Span(), "sequence pattern has multiple stars")
		}
		star = index
	}
	minimum := len(pattern.Elements)
	comparison := uint32(bytecode.CompareEqual)
	if star >= 0 {
		minimum--
		comparison = bytecode.CompareGreaterEqual
	}
	if err := compiler.emit(bytecode.GetLen, 0, pattern.Span()); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.Integer(strconv.Itoa(minimum))),
		pattern.Span(),
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.CompareOp, comparison, pattern.Span()); err != nil {
		return err
	}
	if err := compiler.compilePatternCondition(failed, failureDepth, pattern.Span()); err != nil {
		return err
	}

	if star < 0 {
		if err := compiler.emit(
			bytecode.UnpackSequence,
			uint32(len(pattern.Elements)),
			pattern.Span(),
		); err != nil {
			return err
		}
	} else {
		operand, ok := bytecode.PackUnpackEx(
			uint32(star),
			uint32(len(pattern.Elements)-star-1),
		)
		if !ok {
			return compiler.error(pattern.Span(), "sequence pattern has too many elements")
		}
		if err := compiler.emit(bytecode.UnpackEx, operand, pattern.Span()); err != nil {
			return err
		}
	}
	for _, element := range pattern.Elements {
		if err := compiler.compileExtractingPattern(
			element,
			failed,
			failureDepth,
			captureTemps,
		); err != nil {
			return err
		}
	}
	return nil
}

func (compiler *compilerState) compilePatternCondition(
	failed *jumpLabel,
	failureDepth int,
	span lexer.Span,
) error {
	passed := compiler.newLabel()
	if err := compiler.emitJump(bytecode.PopJumpIfTrue, passed, span); err != nil {
		return err
	}
	if err := compiler.emitPatternFailure(failed, failureDepth, span); err != nil {
		return err
	}
	return compiler.markLabel(passed, span)
}

func (compiler *compilerState) emitPatternFailure(
	failed *jumpLabel,
	failureDepth int,
	span lexer.Span,
) error {
	if compiler.stackDepth < failureDepth {
		return compiler.error(
			span,
			"pattern failure depth %d exceeds stack depth %d",
			failureDepth,
			compiler.stackDepth,
		)
	}
	for compiler.stackDepth > failureDepth {
		if err := compiler.emit(bytecode.PopTop, 0, span); err != nil {
			return err
		}
	}
	return compiler.emitJump(bytecode.Jump, failed, span)
}

func (compiler *compilerState) storePatternCapture(
	name string,
	span lexer.Span,
	captureTemps map[string]uint32,
) error {
	temp, ok := captureTemps[name]
	if !ok {
		return compiler.error(span, "pattern capture %q has no temporary local", name)
	}
	return compiler.emit(bytecode.StoreFast, temp, span)
}

// patternCaptures returns names committed after a successful supported pattern;
// OR alternatives already have equal capture sets from resolution.
func (compiler *compilerState) patternCaptures(
	pattern compilerast.Pattern,
) ([]matchCapture, error) {
	switch pattern := pattern.(type) {
	case *compilerast.ValuePattern, *compilerast.WildcardPattern:
		return nil, nil
	case *compilerast.CapturePattern:
		return []matchCapture{{name: pattern.Name, span: pattern.Span()}}, nil
	case *compilerast.StarPattern:
		if pattern.Name == "" {
			return nil, nil
		}
		return []matchCapture{{name: pattern.Name, span: pattern.Span()}}, nil
	case *compilerast.SequencePattern:
		var captures []matchCapture
		for _, element := range pattern.Elements {
			elementCaptures, err := compiler.patternCaptures(element)
			if err != nil {
				return nil, err
			}
			captures = append(captures, elementCaptures...)
		}
		return captures, nil
	case *compilerast.AsPattern:
		var captures []matchCapture
		var err error
		if pattern.Pattern != nil {
			captures, err = compiler.patternCaptures(pattern.Pattern)
			if err != nil {
				return nil, err
			}
		}
		if pattern.Name != "" {
			captures = append(captures, matchCapture{
				name: pattern.Name,
				span: pattern.Span(),
			})
		}
		return captures, nil
	case *compilerast.OrPattern:
		if len(pattern.Patterns) == 0 {
			return nil, compiler.error(pattern.Span(), "OR pattern has no alternatives")
		}
		for _, alternative := range pattern.Patterns {
			if _, err := compiler.patternCaptures(alternative); err != nil {
				return nil, err
			}
		}
		return compiler.patternCaptures(pattern.Patterns[0])
	default:
		return nil, compiler.unsupported(pattern)
	}
}

// patternNeedsExtraction reports whether a pattern can capture a value other
// than the complete match subject and therefore needs hidden temporary locals.
func patternNeedsExtraction(pattern compilerast.Pattern) bool {
	switch pattern := pattern.(type) {
	case *compilerast.SequencePattern, *compilerast.StarPattern:
		return true
	case *compilerast.AsPattern:
		return pattern.Pattern != nil && patternNeedsExtraction(pattern.Pattern)
	case *compilerast.OrPattern:
		for _, alternative := range pattern.Patterns {
			if patternNeedsExtraction(alternative) {
				return true
			}
		}
	}
	return false
}

func (compiler *compilerState) allocateMatchTemps(
	captures []matchCapture,
) map[string]uint32 {
	temps := make(map[string]uint32, len(captures))
	for _, capture := range captures {
		name := fmt.Sprintf(".match.%d", len(compiler.locals))
		compiler.addLocal(name)
		temps[capture.name] = compiler.localIDs[name]
	}
	return temps
}
