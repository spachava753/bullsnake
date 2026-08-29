package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

// compileFunctionAnnotations creates the lazy PEP 649 annotation callable and
// leaves it below the function body payload for later attribute attachment.
func (compiler *compilerState) compileFunctionAnnotations(
	statement *compilerast.FunctionDefStmt,
) (bool, error) {
	scope := compiler.table.ScopeFor(statement, resolver.Annotations, 0)
	if scope == nil || scope.Kind != resolver.AnnotationScope {
		return false, compiler.error(statement.Span(), "resolver has no annotation scope for %q", statement.Name)
	}
	if scope.Flags&resolver.UsesAnnotations == 0 {
		return false, nil
	}
	if compiler.table.Features&resolver.FutureAnnotations != 0 {
		return false, compiler.error(statement.Span(), "future function annotations are not compiled")
	}

	child := compiler.newAnnotationCompiler(statement, scope)
	if err := child.emitAnnotationFormatGuard(statement.Span()); err != nil {
		return false, err
	}
	count, err := child.compileParameterAnnotations(statement.Parameters)
	if err != nil {
		return false, err
	}
	if statement.Returns != nil {
		if err := child.compileAnnotationEntry("return", statement.Returns); err != nil {
			return false, err
		}
		count++
	}
	if err := child.emit(bytecode.BuildMap, uint32(count), statement.Span()); err != nil {
		return false, err
	}
	if err := child.emitTerminator(bytecode.ReturnValue, 0, statement.Span()); err != nil {
		return false, err
	}
	code, err := child.finish()
	if err != nil {
		return false, err
	}
	if err := compiler.emitFunction(code, false, false, false, statement.Span()); err != nil {
		return false, err
	}
	return true, nil
}

func (compiler *compilerState) newAnnotationCompiler(
	statement *compilerast.FunctionDefStmt,
	scope *resolver.Scope,
) *compilerState {
	flags := bytecode.Optimized | bytecode.NewLocals
	if scope.Flags&resolver.Nested != 0 {
		flags |= bytecode.Nested
	}
	child := &compilerState{
		filename:            compiler.filename,
		module:              compiler.module,
		owner:               statement,
		table:               compiler.table,
		scope:               scope,
		codeName:            "__annotate__",
		qualifiedName:       compiler.childQualifiedName(statement.Name) + ".__annotate__",
		firstLine:           statement.Span().Start.Line,
		codeFlags:           flags,
		positionalOnlyCount: 1,
		positionalCount:     1,
		localIDs:            make(map[string]uint32),
		derefIDs:            make(map[string]uint32),
		constantIDs:         make(map[bytecode.Constant]uint32),
		nameIDs:             make(map[string]uint32),
		reachable:           true,
	}
	// CPython hides the synthetic ".format" binding from source expressions but
	// exposes it as "format" in callable metadata.
	child.addLocal("format")
	child.initializeScopeLayout(scope)
	return child
}

// emitAnnotationFormatGuard accepts VALUE and the internal fake-globals mode,
// then raises NotImplementedError so annotationlib can handle higher formats.
func (compiler *compilerState) emitAnnotationFormatGuard(span lexer.Span) error {
	body := compiler.newLabel()
	if err := compiler.emit(bytecode.LoadFast, 0, span); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.Integer("2")),
		span,
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.CompareOp, bytecode.CompareGreater, span); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.PopJumpIfFalse, body, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.LoadNotImplementedError, 0, span); err != nil {
		return err
	}
	if err := compiler.emitTerminator(bytecode.RaiseVarargs, 1, span); err != nil {
		return err
	}
	return compiler.markLabel(body, span)
}

// compileParameterAnnotations follows CPython 3.14's observable dictionary
// order: ordinary positional parameters precede positional-only parameters.
func (compiler *compilerState) compileParameterAnnotations(
	parameters compilerast.Parameters,
) (int, error) {
	count := 0
	for _, group := range [][]compilerast.Parameter{
		parameters.Positional,
		parameters.PositionalOnly,
	} {
		for _, parameter := range group {
			if parameter.Annotation == nil {
				continue
			}
			if err := compiler.compileAnnotationEntry(parameter.Name, parameter.Annotation); err != nil {
				return 0, err
			}
			count++
		}
	}
	if parameters.VarArg != nil && parameters.VarArg.Annotation != nil {
		if err := compiler.compileAnnotationEntry(
			parameters.VarArg.Name,
			parameters.VarArg.Annotation,
		); err != nil {
			return 0, err
		}
		count++
	}
	for _, parameter := range parameters.KeywordOnly {
		if parameter.Annotation == nil {
			continue
		}
		if err := compiler.compileAnnotationEntry(parameter.Name, parameter.Annotation); err != nil {
			return 0, err
		}
		count++
	}
	if parameters.KeywordVarArg != nil && parameters.KeywordVarArg.Annotation != nil {
		if err := compiler.compileAnnotationEntry(
			parameters.KeywordVarArg.Name,
			parameters.KeywordVarArg.Annotation,
		); err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

func (compiler *compilerState) compileAnnotationEntry(name string, expression compilerast.Expr) error {
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.TextString(name)),
		expression.Span(),
	); err != nil {
		return err
	}
	return compiler.compileExpr(expression)
}
