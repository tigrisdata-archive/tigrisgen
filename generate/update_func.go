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
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/rs/zerolog/log"
	"github.com/tigrisdata/tigrisgen/expr"
	"github.com/tigrisdata/tigrisgen/marshal/tigris"
	"github.com/tigrisdata/tigrisgen/util"
	"golang.org/x/tools/go/packages"
)

var ErrOnlyClientSideAllowed = fmt.Errorf("only client side evaluated conditions allowed in the update function")

func parseUpdateFunction(name string, fn *ast.FuncDecl, pi *packages.Package) (string, string) {
	log.Debug().Str("name", name).Msg("parsing update function")

	if fn.Type.Results != nil {
		util.Fatal("Update should not return results")
	}

	if len(fn.Body.List) < 1 {
		util.Fatal("Update should contain at least one statement")
	}

	l := fn.Type.Params.List

	if len(l) != 2 && (fn.Recv == nil || len(fn.Recv.List) != 1) {
		util.Fatal("Update function should have two parameters, or be a method of the document type")
	}

	var arg0, arg1 string

	var arg0type, arg1type *types.Struct

	if fn.Recv != nil {
		if len(fn.Recv.List[0].Names) != 1 {
			util.Fatal("Receiver should not be empty")
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

	f := funcParser{
		pi:  pi,
		doc: arg0, docType: arg0type,
		args: arg1, argsType: arg1type,
	}

	log.Debug().Str("param_name", f.doc).Msg("doc")
	log.Debug().Str("param_name", f.args).Msg("args")

	upd := f.parseUpdateBlockStmt(fn.Body)

	return name, tigris.MarshalUpdate(upd)
}

func updOp(op expr.Op, lhs expr.Operand, rhs expr.Operand) expr.Expr {
	return expr.NewExpr(op, lhs, rhs)
}

func (f *funcParser) parseUpdateBlockStmt(block *ast.BlockStmt) []expr.Expr {
	var upd []expr.Expr

	for _, v := range block.List {
		switch e := v.(type) {
		case *ast.AssignStmt:
			log.Debug().Msg("Assignment statement")

			if len(e.Lhs) != 1 {
				util.Fatal("Only one operand is allowed on left hand side")
			}

			if len(e.Rhs) != 1 {
				util.Fatal("Only one operand is allowed on right hand side")
			}

			lhs := f.parseOperand(e.Lhs[0])
			if lhs.Type != expr.Field {
				util.Fatal("Document field is expected on the left hand side")
			}

			switch ee := e.Rhs[0].(type) {
			case *ast.CallExpr:
				fn := f.parseFuncCall(ee)
				if fn.Type == expr.PushOp {
					if lhs.Value.(string) == fn.X.Value.(string) {
						upd = append(upd, fn)
						continue
					}
				} else if fn.Type == expr.TimeNow {
					upd = append(upd, updOp(expr.SetOp, lhs, expr.NewOperand("{{toJSON .Time}}", expr.Arg)))
					continue
				}

				FatalWithExpr(f.pi, e, "Unsupported update statement")
			}

			rhs := f.parseOperand(e.Rhs[0])
			if rhs.Type != expr.Constant && rhs.Type != expr.Arg {
				util.Fatal("Arguments field is expected on the right hand side")
			}

			switch e.Tok {
			case token.ADD_ASSIGN: // +=
				upd = append(upd, updOp(expr.IncOp, lhs, rhs))
			case token.SUB_ASSIGN: // -=
				upd = append(upd, updOp(expr.DecOp, lhs, rhs))
			case token.MUL_ASSIGN: // *=
				upd = append(upd, updOp(expr.MulOp, lhs, rhs))
			case token.QUO_ASSIGN: // /=
				upd = append(upd, updOp(expr.DivOp, lhs, rhs))
			case token.ASSIGN: // =
				upd = append(upd, updOp(expr.SetOp, lhs, rhs))
			case token.INC: // ++
				upd = append(upd, updOp(expr.IncOp, lhs, expr.NewConstant(1)))
			case token.DEC: // --
				upd = append(upd, updOp(expr.DecOp, lhs, expr.NewConstant(1)))
			default:
				FatalWithExpr(f.pi, e, "Unsupported assignment operator")
			}

		case *ast.IfStmt:
			ifCond, ifBody := f.parseUpdateIfStatement(e)

			upd = append(upd, expr.NewUpdIfExpr(expr.UpdIfOp, ifCond, ifBody))
		default:
			FatalWithExpr(f.pi, e, "Unsupported update statement")
		}
	}

	return upd
}

func (f *funcParser) parseUpdateIfStatement(stmt *ast.IfStmt) (expr.Expr, []expr.Expr) {
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
			FatalWithExpr(f.pi, e, ErrOnlyClientSideAllowed.Error())
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
			FatalWithExpr(f.pi, e, ErrOnlyClientSideAllowed.Error())
		} else if x.Type == expr.Arg {
			ifCond = expr.NewExpr(expr.Eq, x, expr.NewConstant(true)).Client()
		} else {
			FatalWithExpr(f.pi, e, "unsupported selector in if condition")
		}
	case *ast.UnaryExpr:
		ifCond = f.parseUnaryNegation(e.X)
	default:
		FatalWithExpr(f.pi, e, "unsupported statement if statement")
	}

	ifBody := f.parseUpdateBlockStmt(stmt.Body)

	if stmt.Else == nil {
		return ifCond, ifBody
	}

	switch e := stmt.Else.(type) {
	case *ast.IfStmt:
		elseCond, elseBody := f.parseUpdateIfStatement(e)
		return expr.And(expr.Negate(ifCond), elseCond), elseBody
	case *ast.BlockStmt:
		elseBody := f.parseUpdateBlockStmt(e)
		return expr.Negate(ifCond), elseBody
	default:
		FatalWithExpr(f.pi, e, "unknown else statement")
	}

	return expr.Expr{}, nil
}
