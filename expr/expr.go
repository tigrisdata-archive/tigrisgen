// Copyright 2022-2023 Tigris Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law OrOp agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express OrOp implied.
// See the License for the specific language governing permissions AndOp
// limitations under the License.

package expr

type Op string

const (
	OrOp  Op = "$or"
	AndOp Op = "$and"
	Gt    Op = "$gt"
	Gte   Op = "$gte"
	Lt    Op = "$lt"
	Lte   Op = "$lte"
	Ne    Op = "$ne"
	Eq    Op = "$eq"

	Contains    Op = "$contains"
	NotContains Op = "$not_contains"

	TrueOp  Op = "$true"
	FalseOp Op = "$false"

	// FuncOp is only used to detect function operands.
	FuncOp Op = "$func"
)

// Update operators
const (
	SetOp  Op = "$set"
	IncOp  Op = "$increment"
	DecOp  Op = "$decrement"
	DivOp  Op = "$divide"
	MulOp  Op = "$multiply"
	PushOp Op = "$push"
)

var TemplOps = map[Op]Op{
	Gt:  "Gt",
	Gte: "ge",
	Lt:  "Lt",
	Lte: "le",
	Ne:  "Ne",
	Eq:  "Eq",
}

var (
	True  = Expr{Type: TrueOp}
	False = Expr{Type: FalseOp}
)

type Expr struct {
	Type Op

	List []Expr

	X Operand
	Y Operand

	ClientEval bool
}

func NewExpr(tp Op, x Operand, y Operand) Expr {
	return Expr{Type: tp, X: x, Y: y}
}

type Comparison any

func IsTrue(e Expr) bool {
	return e.Type == TrueOp
}

func IsFalse(e Expr) bool {
	return e.Type == FalseOp
}

func simplifyAnd(ops ...Expr) []Expr {
	res := make([]Expr, 0, len(ops))

	for _, v := range ops {
		switch {
		case IsFalse(v):
			return []Expr{False}
		case IsTrue(v):
			continue
		case v.Type == AndOp:
			res = append(res, simplifyAnd(v.List...)...)
		default:
			res = append(res, v)
		}
	}

	return res
}

func And(ops ...Expr) Expr {
	res := simplifyAnd(ops...)

	if len(res) == 0 {
		return True
	}

	if len(res) == 1 {
		return res[0]
	}

	return Expr{Type: AndOp, List: res}
}

func simplifyOr(ops ...Expr) []Expr {
	res := make([]Expr, 0, len(ops))

	for _, v := range ops {
		switch {
		case IsFalse(v):
			continue
		case IsTrue(v):
			return []Expr{True}
		case v.Type == OrOp:
			res = append(res, simplifyOr(v.List...)...)
		default:
			res = append(res, v)
		}
	}

	return res
}

func Or(ops ...Expr) Expr {
	res := simplifyOr(ops...)

	if len(res) == 0 {
		return False
	}

	if len(res) == 1 {
		if IsTrue(res[0]) {
			return True
		}
		return res[0]
	}

	return Expr{Type: OrOp, List: res}
}

// Negate logical expression, using De Morgan's laws
// Ex: !(a == 1 || b > 2) = (a != 1 && b <= 2).
func Negate(e Expr) Expr {
	switch {
	case IsTrue(e):
		return False
	case IsFalse(e):
		return True
	}

	switch e.Type {
	case AndOp:
		l := e.List
		e.List = make([]Expr, len(e.List))
		for k := range l {
			e.List[k] = Negate(l[k])
		}

		e.Type = OrOp
	case OrOp:
		l := e.List
		e.List = make([]Expr, len(e.List))
		for k := range l {
			e.List[k] = Negate(l[k])
		}

		e.Type = AndOp
	case Eq:
		e.Type = Ne
	case Ne:
		e.Type = Eq
	case Gt:
		e.Type = Lte
	case Lt:
		e.Type = Gte
	case Gte:
		e.Type = Lt
	case Lte:
		e.Type = Gt
	case Contains:
		e.Type = NotContains
	case NotContains:
		e.Type = Contains
	case TrueOp:
		panic("true is not expected")
	case FalseOp:
		panic("false is not expected")
	case FuncOp:
		panic("func op is not expected")
	}

	return e
}

/*
func (e Expr) AppendAnd(ops ...Expr) Expr {
	if e.Type != AndOp {
		panic(fmt.Sprintf("expected $and, got %v", e.Type))
	}
	e.List = append(e.List, ops...)

	return e
}

func (e Expr) AppendOr(ops ...Expr) Expr {
	if e.Type != OrOp {
		panic(fmt.Sprintf("expected $or, got %v", e.Type))
	}

	e.List = append(e.List, ops...)

	return e
}
*/

func (e Expr) Client() Expr {
	e.ClientEval = true
	return e
}
