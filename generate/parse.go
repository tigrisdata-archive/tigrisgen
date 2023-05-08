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
	"bytes"
	_ "embed"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"reflect"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/tigrisdata/tigrisgen/expr"
	"github.com/tigrisdata/tigrisgen/util"
	"golang.org/x/tools/go/packages"
)

type funcParser struct {
	doc      string
	docType  *types.Struct
	args     string
	argsType *types.Struct
	pi       *packages.Package
}

func parseConst(v constant.Value) expr.Operand {
	switch v.Kind() {
	case constant.Bool:
		return expr.NewOperand(constant.BoolVal(v), expr.Constant)
	case constant.String:
		return expr.NewOperand(constant.StringVal(v), expr.Constant)
	case constant.Int:
		i, ok := constant.Int64Val(v)
		if !ok {
			util.Fatal("unsupported contant integer value: %v", v.ExactString())
		}

		return expr.NewOperand(i, expr.Constant)
	case constant.Float:
		f, ok := constant.Float64Val(v)
		if !ok {
			util.Fatal("unsupported contant integer value: %v", v.ExactString())
		}

		return expr.NewOperand(f, expr.Constant)
	default:
		util.Fatal("unsupported constant value: %v", v.ExactString())
	}

	return expr.Operand{} // unreachable
}

func (f *funcParser) parseTrueFalseOnly(e ast.Expr) expr.Expr {
	if v := f.pi.TypesInfo.Types[e].Value; v != nil {
		c := parseConst(v)
		if b, ok := c.Value.(bool); ok {
			if b {
				return expr.True
			}

			return expr.False
		}
	}

	FatalWithExpr(f.pi, e, "unsupported constant in unary operator")
	panic("unsupported constant in unary operator")
}

// creates field name in the form of path elements from the selector.
func (f *funcParser) parseSelector(in ast.Expr) (string, []string) {
	cnt := 0
	ex := in

	// count the number of elements in path
L:
	for {
		switch e := ex.(type) {
		case *ast.SelectorExpr:
			ex = e.X
		case *ast.IndexExpr:
			ex = e.X
		case *ast.Ident:
			break L
		default:
			FatalWithExpr(f.pi, in, "unknown expr in selector")
		}

		cnt++
	}

	path := make([]string, cnt)

	// saves the path in proper order
	for cnt--; ; cnt-- {
		switch e := in.(type) {
		case *ast.SelectorExpr:
			path[cnt] = e.Sel.Name
			in = e.X
		case *ast.IndexExpr:
			x := f.parseOperand(e.Index)
			if x.Type == expr.Constant {
				path[cnt] = fmt.Sprintf("%v", x.Value)
			} else if x.Type == expr.Arg {
				path[cnt] = fmt.Sprintf("{{.Arg.%v}}", x.Value)
			}

			in = e.X
		case *ast.Ident:
			return e.Name, path
		}
	}
}

// toFieldName creates dotted field name from "path".
// It goes down the struct type, replacing path element
// with corresponding field JSON tag, otherwise uses the name as is.
func toFieldName(tp *types.Struct, path []string) string {
	var (
		sb       bytes.Buffer
		mapOrArr bool
	)

	for _, f := range path {
		if tp == nil {
			util.Fatal("nested field not found: %v, path: %+v", f, path)
		}

		if mapOrArr {
			if sb.Len() != 0 {
				sb.WriteString(".")
			}

			sb.WriteString(f)

			mapOrArr = false

			continue
		}

		for i := 0; i < tp.NumFields(); i++ {
			if tp.Field(i).Name() == f {
				if sb.Len() != 0 {
					sb.WriteString(".")
				}

				tag := strings.Split(reflect.StructTag(tp.Tag(i)).Get("json"), ",")
				if tag[0] != "" {
					sb.WriteString(tag[0])
				} else {
					sb.WriteString(tp.Field(i).Name())
				}

				if _, ok := tp.Field(i).Type().(*types.Named); ok {
					tp, _ = tp.Field(i).Type().(*types.Named).Underlying().(*types.Struct)
				} else if s, ok := tp.Field(i).Type().(*types.Slice); ok {
					if _, ok = s.Elem().Underlying().(*types.Struct); ok {
						tp, _ = s.Elem().Underlying().(*types.Struct)
					}
					mapOrArr = true
				} else if s, ok := tp.Field(i).Type().(*types.Map); ok {
					if _, ok = s.Elem().Underlying().(*types.Struct); ok {
						tp, _ = s.Elem().Underlying().(*types.Struct)
					}
					mapOrArr = true
				}

				break
			}
		}
	}

	return sb.String()
}

func (f *funcParser) parseOperand(node ast.Expr) expr.Operand {
	log.Debug().Msg("parse operand")

	if v := f.pi.TypesInfo.Types[node].Value; v != nil {
		return parseConst(v)
	}

	switch e := node.(type) {
	case *ast.SelectorExpr, *ast.IndexExpr:
		n, path := f.parseSelector(e)

		if n != f.doc && n != f.args {
			util.Fatal("unsupported selector %+v, expected: %v or %v", n, f.doc, f.args)
		}

		if n == f.doc {
			return expr.NewOperand(toFieldName(f.docType, path), expr.Field)
		}

		return expr.NewOperand(strings.Join(path, "."), expr.Arg) // struct arg
	case *ast.Ident:
		switch e.Name {
		case f.args:
			return expr.NewOperand("", expr.Arg) // simple arg
		}
	case *ast.CallExpr:
		ee := f.parseFuncCall(e)
		if ee.Type == expr.FuncOp {
			return expr.NewFunc(ee.X, ee.Y)
		}
	}

	FatalWithExpr(f.pi, node, "unsupported operand type")

	return expr.NewOperand(nil, 0) // unreachable
}

func (f *funcParser) parseUnaryNegation(e ast.Expr) expr.Expr {
	if v := f.pi.TypesInfo.Types[e].Value; v != nil {
		return expr.Negate(f.parseTrueFalseOnly(e))
	}

	switch ee := e.(type) {
	case *ast.BinaryExpr:
		return expr.Negate(f.parseBinaryExprLow(ee))
	case *ast.SelectorExpr, *ast.IndexExpr:
		x := f.parseOperand(ee)
		if x.Type == expr.Field {
			return expr.NewExpr(expr.Ne, x, expr.NewConstant(true))
		} else if x.Type == expr.Arg {
			return expr.NewExpr(expr.Ne, x, expr.NewConstant(true)).Client()
		}
	case *ast.Ident:
		x := f.parseOperand(ee)
		if x.Type == expr.Arg {
			return expr.NewExpr(expr.Ne, x, expr.NewConstant(true)).Client()
		}
	case *ast.CallExpr:
		x := f.parseFuncCall(ee)
		return expr.Negate(x)
	}

	FatalWithExpr(f.pi, e, "unsupported unary operator")

	return expr.Expr{} // unreachable
}

func (f *funcParser) parseFuncCall(e *ast.CallExpr) expr.Expr {
	log.Debug().Msg("parse func call")

	switch fn := e.Fun.(type) {
	case *ast.SelectorExpr:
		s, ok := fn.X.(*ast.Ident)
		if !ok {
			if len(e.Args) != 1 {
				break
			}

			tt, ok := f.pi.TypesInfo.Types[fn.X].Type.(*types.Named)
			if ok && tt.String() == "time.Time" {
				x := f.parseOperand(fn.X)
				y := f.parseOperand(e.Args[0])

				expr.ValidateOperands(x, y, nil)

				switch fn.Sel.Name {
				case "After":
					log.Debug().Str("op", string(expr.Gt)).
						Interface("x", x.Value).Interface("y", y.Value).Msg("time.After")

					return filterOp(expr.Gt, x, y)
				case "Before":
					log.Debug().Str("op", string(expr.Lt)).
						Interface("x", x.Value).Interface("y", y.Value).Msg("time.After")

					return filterOp(expr.Lt, x, y)
				case "Equal":
					log.Debug().Str("op", string(expr.Eq)).
						Interface("x", x.Value).Interface("y", y.Value).Msg("time.After")

					return filterOp(expr.Eq, x, y)
				case "Compare":
					log.Debug().Str("op", "Compare").
						Interface("x", x.Value).Interface("y", y.Value).Msg("time.After")

					return expr.NewExpr(expr.FuncOp, x, y)
				}
			}

			break
		}

		if pkg, ok := f.pi.TypesInfo.ObjectOf(s).(*types.PkgName); ok {
			path := pkg.Imported().Path()
			switch path {
			case "strings":
				if fn.Sel.Name == "Contains" {
					x := f.parseOperand(e.Args[0])
					y := f.parseOperand(e.Args[1])
					expr.ValidateOperands(x, y, nil)

					return filterOp(expr.Contains, x, y)
				}
			case "bytes":
				if fn.Sel.Name == "Compare" {
					x := f.parseOperand(e.Args[0])
					y := f.parseOperand(e.Args[1])
					expr.ValidateOperands(x, y, nil)

					return expr.NewExpr(expr.FuncOp, x, y) // this is further processed in filterOp
				}
			case "time":
				switch fn.Sel.Name {
				case "Now":
					return expr.NewExpr(expr.TimeNow, expr.NewOperand(nil, expr.Func),
						expr.NewOperand(nil, expr.Func))
				}
			}
		}
	case *ast.Ident:
		if fn.Name == "append" {
			x := f.parseOperand(e.Args[0])
			y := f.parseOperand(e.Args[1])
			expr.ValidateOperands(x, y, nil)

			if x.Type == expr.Field && (y.Type == expr.Constant || y.Type == expr.Arg) {
				return expr.NewExpr(expr.PushOp, x, y)
			}
		}
	}

	FatalWithExpr(f.pi, e, "unsupported function call")

	return expr.Expr{}
}

func filterOp(op expr.Op, x expr.Operand, y expr.Operand) expr.Expr {
	if x.Type == expr.Field {
		return expr.NewExpr(op, x, y)
	}

	if y.Type == expr.Field {
		if op != expr.Eq {
			return expr.Negate(expr.NewExpr(op, y, x))
		}

		return expr.NewExpr(op, y, x)
	}

	if x.Type == expr.Func {
		return filterOp(op, x.Value.(expr.Operand), x.Value1.(expr.Operand))
	}

	if y.Type == expr.Func {
		return filterOp(op, y.Value.(expr.Operand), y.Value1.(expr.Operand))
	}

	// Args and Constant
	return expr.NewExpr(op, x, y).Client()
}

func (f *funcParser) parseBinaryExprLow(node ast.Node) expr.Expr {
	switch e := node.(type) {
	case *ast.BinaryExpr:
		log.Debug().Str("op", e.Op.String()).Msg("parse binary expression")

		switch e.Op {
		case token.LAND:
			x := f.parseBinaryExprLow(e.X)
			y := f.parseBinaryExprLow(e.Y)

			return expr.And(x, y)
		case token.LOR:
			x := f.parseBinaryExprLow(e.X)
			y := f.parseBinaryExprLow(e.Y)

			return expr.Or(x, y)
		case token.GEQ:
			x := f.parseOperand(e.X)
			y := f.parseOperand(e.Y)
			expr.ValidateOperands(x, y, e)

			return filterOp(expr.Gte, x, y)
		case token.LEQ:
			x := f.parseOperand(e.X)
			y := f.parseOperand(e.Y)
			expr.ValidateOperands(x, y, e)

			return filterOp(expr.Lte, x, y)
		case token.LSS:
			x := f.parseOperand(e.X)
			y := f.parseOperand(e.Y)
			expr.ValidateOperands(x, y, e)

			return filterOp(expr.Lt, x, y)
		case token.GTR:
			x := f.parseOperand(e.X)
			y := f.parseOperand(e.Y)
			expr.ValidateOperands(x, y, e)

			return filterOp(expr.Gt, x, y)
		case token.EQL:
			x := f.parseOperand(e.X)
			y := f.parseOperand(e.Y)
			expr.ValidateOperands(x, y, e)

			return filterOp(expr.Eq, x, y)
		case token.NEQ:
			x := f.parseOperand(e.X)
			y := f.parseOperand(e.Y)
			expr.ValidateOperands(x, y, e)

			return filterOp(expr.Ne, x, y)
		default:
			util.Fatal("unsupported binary op: %v", e.Op.String())
		}
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			return f.parseUnaryNegation(e.X)
		}
	case *ast.Ident: // only true | false idents supported
		x := f.parseOperand(e)
		if x.Type == expr.Arg {
			return expr.NewExpr(expr.Eq, x, expr.NewConstant(true)).Client()
		}

		return f.parseTrueFalseOnly(e)
	case *ast.SelectorExpr:
		x := f.parseOperand(e)
		if x.Type == expr.Field {
			return expr.NewExpr(expr.Eq, x, expr.NewConstant(true))
		} else if x.Type == expr.Arg {
			return expr.NewExpr(expr.Eq, x, expr.NewConstant(true)).Client()
		}
	case *ast.BasicLit:
		return f.parseTrueFalseOnly(e)
	case *ast.ParenExpr:
		if ee, ok := e.X.(*ast.BinaryExpr); ok {
			return f.parseBinaryExprLow(ee)
		}
	case *ast.CallExpr:
		return f.parseFuncCall(e)
	}

	FatalWithExpr(f.pi, node, "unexpected binary expression")
	panic("unexpected expression")
}

func (f *funcParser) parseBinaryExpr(expr *ast.BinaryExpr) expr.Expr {
	return f.parseBinaryExprLow(expr)
}

func (f *funcParser) parseReturnStatement(stmt *ast.ReturnStmt) expr.Expr {
	log.Debug().Msg("parseReturnStatement")

	if len(stmt.Results) != 1 {
		util.Fatal("Only one bool result is allowed in return")
	}

	switch e := stmt.Results[0].(type) {
	case *ast.BinaryExpr:
		return f.parseBinaryExpr(e)
	case *ast.Ident:
		x := f.parseOperand(e)
		if x.Type == expr.Field {
			return expr.NewExpr(expr.Eq, x, expr.NewConstant(true))
		} else if x.Type == expr.Constant {
			if b, ok := x.Value.(bool); ok {
				if b {
					return expr.True
				}

				return expr.False
			}
		}

		util.Fatal("unsupported return variable: %+v", e)
	case *ast.SelectorExpr:
		x := f.parseOperand(e)
		if x.Type == expr.Field {
			return expr.NewExpr(expr.Eq, x, expr.NewConstant(true))
		} else {
			util.Fatal("unsupported return variable: %+v", e)
		}
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			return f.parseUnaryNegation(e.X)
		}
	case *ast.CallExpr:
		return f.parseFuncCall(e)
	default:
		FatalWithExpr(f.pi, e, "return should be a logical expression")
	}

	panic("unsupported return statement")
}
