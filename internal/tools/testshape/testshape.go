// Package testshape reports Test functions that loop, open subtests or do not
// end in exactly one assertion.
package testshape

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

// Analyzer is the testshape analysis.
var Analyzer = &analysis.Analyzer{
	Name:     "testshape",
	Doc:      "reports Test functions with loops, t.Run or a number of assertions other than one",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if !isTest(pass, fn) {
			return
		}
		checkTest(pass, fn)
	})
	return nil, nil
}

func isTest(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	file := pass.Fset.File(fn.Pos())
	return fn.Recv == nil &&
		strings.HasSuffix(file.Name(), "_test.go") &&
		strings.HasPrefix(fn.Name.Name, "Test")
}

func checkTest(pass *analysis.Pass, fn *ast.FuncDecl) {
	assertions := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			pass.Reportf(node.Pos(), "%s: loop in test; write one test per scenario", fn.Name.Name)
		case *ast.CallExpr:
			callee := typeutil.Callee(pass.TypesInfo, node)
			if isSubtest(callee) {
				pass.Reportf(node.Pos(), "%s: t.Run in test; write one test per scenario", fn.Name.Name)
			}
			if isAssertion(callee) {
				assertions++
			}
		}
		return true
	})
	if assertions != 1 {
		pass.Reportf(fn.Pos(), "%s: %d assertions, want exactly 1", fn.Name.Name, assertions)
	}
}

func isSubtest(obj types.Object) bool {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "testing" || fn.Name() != "Run" {
		return false
	}
	return fn.Signature().Recv() != nil
}

func isAssertion(obj types.Object) bool {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil {
		return false
	}
	path, name := fn.Pkg().Path(), fn.Name()
	if strings.HasSuffix(path, "internal/testkit") {
		return name == "Equal" || name == "ErrorIs"
	}
	return path == "golang.org/x/tools/go/analysis/analysistest" && name == "Run"
}
