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

package expr

import (
	"go/ast"

	"github.com/tigrisdata/tigrisgen/util"
)

type OperandType int

const (
	_ OperandType = iota
	Field
	Constant
	Arg
	Func
)

type Operand struct {
	Type   OperandType
	Value  any
	Value1 any
}

func NewOperand(val any, typ OperandType) Operand {
	return Operand{Type: typ, Value: val}
}

func NewConstant(val any) Operand {
	return Operand{Type: Constant, Value: val}
}

func NewField(val any) Operand {
	return Operand{Type: Field, Value: val}
}

func NewArg(val any) Operand {
	return Operand{Type: Arg, Value: val}
}

func NewFunc(arg any, arg1 any) Operand {
	return Operand{Type: Func, Value: arg, Value1: arg1}
}

func ValidateOperands(x Operand, y Operand, e *ast.BinaryExpr) {
	if x.Type == Constant && y.Type == Constant ||
		x.Type == Field && y.Type == Field ||
		x.Type == Arg && y.Type == Arg ||
		x.Type == Func && y.Type == Func ||
		x.Type == Func && y.Type != Constant ||
		y.Type == Func && x.Type != Constant {
		if e != nil {
			util.Fatal("field name, arg, func call or constant expected in binary operation, got: %v %v %v", e.X, e.Op, e.Y)
		}

		util.Fatal("field name, arg, func call or constant expected in binary operation")
	}
}
