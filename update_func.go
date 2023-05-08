package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"github.com/tigrisdata/tigris-client-go/tigrisgen/marshal/tigris"

	"github.com/tigrisdata/tigris-client-go/tigrisgen/expr"

	"github.com/tigrisdata/tigris-client-go/tigrisgen/util"

	"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog/log"
	"golang.org/x/tools/go/loader"
)

func parseFuncParamType(field *ast.Field) string {
	var param *ast.Ident

	if e, ok := field.Type.(*ast.StarExpr); ok {
		param, ok = e.X.(*ast.Ident)
		if !ok {
			util.Fatal("unsupported parameter type: %v", reflect.TypeOf(e.X))
		}
		if param.Obj != nil {
			spew.Dump(param.Obj.Decl)
		}
	} else if e, ok := field.Type.(*ast.Ident); ok {
		param = e
		/*
			if param.Obj != nil {
				spew.Dump(param.Obj.Decl)
			}
		*/
	} else {
		util.Fatal("unsupported parameter type: %v", reflect.TypeOf(e))
	}

	//	log.Debug().Str("param_name", field.Names[0].X).Msg("doc")

	return param.Name
}

func parseUpdateFunction(name string, fn *ast.FuncDecl, pi *loader.PackageInfo) (string, string) {
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
				FatalWithExpr(e, "Unsupported assignment operator")
			}

		default:
			FatalWithExpr(e, "Unsupported update statement")
		}
	}

	return upd
}
