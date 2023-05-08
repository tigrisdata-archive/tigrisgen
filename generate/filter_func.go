// Copyright 2022-2023 Tigris Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generate

import (
	"go/ast"
	"go/types"

	"github.com/rs/zerolog/log"
	"github.com/tigrisdata/tigrisgen/expr"
	filter "github.com/tigrisdata/tigrisgen/marshal/tigris"
	"github.com/tigrisdata/tigrisgen/util"
	"golang.org/x/tools/go/packages"
)

func (f *funcParser) parseIfStatement(stmt *ast.IfStmt) (expr.Expr, *expr.Expr) {
	log.Debug().Msg("parseIfStatement")

	if stmt.Init != nil {
		util.Fatal("if init section is not supported")
	}

	var ifCond expr.Expr

	switch e := stmt.Cond.(type) {
	case *ast.BinaryExpr:
		ifCond = f.parseBinaryExpr(e)
	case *ast.Ident: // if true/false/bool field/bool arg {}
		x := f.parseOperand(e)
		if x.Type == expr.Field {
			ifCond = expr.NewExpr(expr.Eq, x, expr.NewConstant(true))
		} else if x.Type == expr.Arg {
			ifCond = expr.NewExpr(expr.Eq, x, expr.NewConstant(true)).Client()
		} else if x.Type == expr.Constant {
			b, ok := x.Value.(bool)
			if !ok {
				FatalWithExpr(f.pi, e, "unsupported constant in if")
			}
			if b {
				ifCond = expr.True
			} else {
				ifCond = expr.False
			}
		}
	case *ast.SelectorExpr: // if doc.Field {}
		x := f.parseOperand(e)
		if x.Type == expr.Field {
			ifCond = expr.NewExpr(expr.Eq, x, expr.NewConstant(true))
		} else if x.Type == expr.Arg {
			ifCond = expr.NewExpr(expr.Eq, x, expr.NewConstant(true)).Client()
		} else {
			FatalWithExpr(f.pi, e, "unsupported select in if condition")
		}
	case *ast.UnaryExpr:
		ifCond = f.parseUnaryNegation(e.X)
	default:
		FatalWithExpr(f.pi, e, "unsupported statement if statement")
	}

	ifBody, ifBodyFallThrough := f.parseBlockStmt(stmt.Body)

	ifExpr := expr.And(ifCond, ifBody)

	if stmt.Else == nil {
		if ifBodyFallThrough == nil {
			b := expr.Negate(ifCond)
			return ifExpr, &b
		}

		b := expr.Or(expr.Negate(ifCond), *ifBodyFallThrough)

		return ifExpr, &b
	}

	var (
		elseExpr        expr.Expr
		elseFallThrough *expr.Expr
	)

	switch e := stmt.Else.(type) {
	case *ast.IfStmt:
		elseExpr, elseFallThrough = f.parseIfStatement(e)
	case *ast.BlockStmt:
		elseExpr, elseFallThrough = f.parseBlockStmt(e)
	}

	ifExpr = expr.Or(ifExpr, expr.And(expr.Negate(ifCond), elseExpr))

	if ifBodyFallThrough == nil && elseFallThrough == nil {
		return ifExpr, nil
	}

	if ifBodyFallThrough == nil {
		b := expr.And(expr.Negate(ifCond), *elseFallThrough)
		return ifExpr, &b
	}

	if elseFallThrough == nil {
		b := expr.And(ifCond, *ifBodyFallThrough)
		return ifExpr, &b
	}

	b := expr.Or(
		expr.And(ifCond, *ifBodyFallThrough),
		expr.And(expr.Negate(ifCond), *elseFallThrough),
	)

	return ifExpr, &b
}

func (f *funcParser) parseBlockStmtLow(block []ast.Stmt) (expr.Expr, *expr.Expr) {
	log.Debug().Msg("parseBlockStatement")

	i := 0
	for ; i < len(block); i++ {
		v := block[i]

		switch e := v.(type) {
		case *ast.ReturnStmt:
			if i < len(block)-1 {
				FatalWithExpr(f.pi, block[i+1], "unreachable code")
			}

			return f.parseReturnStatement(e), nil
		case *ast.IfStmt:
			ifExpr, ifFallThrough := f.parseIfStatement(e)

			if ifFallThrough == nil && i < len(block)-1 {
				FatalWithExpr(f.pi, block[i+1], "unreachable code")
			}

			if ifFallThrough == nil {
				return ifExpr, nil
			}

			if i >= len(block)-1 {
				return ifExpr, ifFallThrough
			}

			restExpr, restFallThrough := f.parseBlockStmtLow(block[i+1:])

			blockExpr := expr.Or(ifExpr, expr.And(*ifFallThrough, restExpr))

			if restFallThrough != nil {
				b := expr.And(*ifFallThrough, *restFallThrough)
				return blockExpr, &b
			}

			return blockExpr, nil
		default:
			FatalWithExpr(f.pi, e, "unsupported block statement")
		}
	}

	FatalWithExpr(f.pi, nil, "unsupported block statement")

	return expr.Expr{}, &expr.Expr{}
}

func (f *funcParser) parseBlockStmt(block *ast.BlockStmt) (expr.Expr, *expr.Expr) {
	return f.parseBlockStmtLow(block.List)
}

func argToType(expr ast.Expr, mustBeStruct bool, pi *packages.Package) *types.Struct {
	t := pi.TypesInfo.Types[expr].Type

	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}

	if t == nil {
		FatalWithExpr(pi, expr, "Argument type not found")
	}

	s, ok := t.Underlying().(*types.Struct)
	if !ok && mustBeStruct {
		FatalWithExpr(pi, expr, "Document parameter should be of struct type, got:")
	}

	return s
}

// returns filter name and filter body parsed from function declaration.
func parseFilterFunction(name string, fn *ast.FuncDecl, pi *packages.Package) (string, string) {
	log.Debug().Str("name", name).Msg("parsing filter function")

	if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 || fn.Type.Results.List[0].Type.(*ast.Ident).Name != "bool" {
		util.Fatal("filter should have bool return type")
	}

	if len(fn.Body.List) < 1 {
		util.Fatal("Return statement is missing in Tigris filter function. Make sure there is no syntax error in the program.")
	}

	l := fn.Type.Params.List

	if len(l) != 2 && (fn.Recv == nil || len(fn.Recv.List) != 1) {
		util.Fatal("Filter function expects exactly two parameters. First is pointer to document type. Second is query arguments")
	}

	var arg0, arg1 string

	var arg0type, arg1type *types.Struct

	if fn.Recv != nil {
		if len(fn.Recv.List[0].Names) != 1 {
			util.Fatal("receiver should not be empty")
		}

		arg0 = fn.Recv.List[0].Names[0].Name
		arg0type = argToType(fn.Recv.List[0].Type, true, pi)

		arg1 = l[0].Names[0].Name
		arg1type = argToType(l[0].Type, false, pi)
	} else {
		arg0 = l[0].Names[0].Name
		arg0type = argToType(l[0].Type, true, pi)
		arg1 = l[1].Names[0].Name
		arg1type = argToType(l[1].Type, false, pi)
	}

	log.Debug().Str("doc", arg0).Str("args", arg1).Msg("params")

	f := funcParser{doc: arg0, args: arg1, docType: arg0type, argsType: arg1type, pi: pi}

	flt, _ := f.parseBlockStmt(fn.Body)

	return name, filter.MarshalFilter(flt)
}
