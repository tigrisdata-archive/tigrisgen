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
	FuncOp  Op = "$func"
	UpdIfOp Op = "$if"
)

// Update operators.
const (
	SetOp  Op = "$set"
	IncOp  Op = "$increment"
	DecOp  Op = "$decrement"
	DivOp  Op = "$divide"
	MulOp  Op = "$multiply"
	PushOp Op = "$push"

	TimeNow Op = "$time_now" // this client side substituted

)

var UpdOps = []Op{SetOp, IncOp, DecOp, DivOp, MulOp, PushOp}

var TemplOps = map[Op]Op{
	Gt:    "gt",
	Gte:   "ge",
	Lt:    "lt",
	Lte:   "le",
	Ne:    "ne",
	Eq:    "eq",
	AndOp: "and",
	OrOp:  "or",
}

var (
	True  = Expr{Type: TrueOp}
	False = Expr{Type: FalseOp}
)

type Expr struct {
	Type Op

	List       []Expr
	ListClient []Expr

	X Operand
	Y Operand

	ClientEval bool
}

func NewExpr(tp Op, x Operand, y Operand) Expr {
	return Expr{Type: tp, X: x, Y: y}
}

func NewUpdIfExpr(tp Op, cond Expr, body []Expr) Expr {
	return Expr{Type: tp, ListClient: []Expr{cond}, List: body}
}

type Comparison any

func IsTrue(e Expr) bool {
	return e.Type == TrueOp
}

func IsFalse(e Expr) bool {
	return e.Type == FalseOp
}

func simplifyAnd(ops ...Expr) ([]Expr, []Expr) {
	var (
		res       = make([]Expr, 0, len(ops))
		resClient = make([]Expr, 0, len(ops))
	)

	for _, v := range ops {
		switch {
		case IsFalse(v):
			return []Expr{False}, nil
		case IsTrue(v):
			continue
		case v.Type == AndOp:
			r, c := simplifyAnd(v.List...)
			if len(r) > 0 {
				res = append(res, r...)
			}

			if len(c) > 0 {
				resClient = append(resClient, c...)
			}

			if len(v.ListClient) > 0 {
				resClient = append(resClient, v.ListClient...)
			}
		case v.Type == OrOp:
			if len(v.List) == 0 {
				resClient = append(resClient, v)
			} else {
				res = append(res, v)
			}
		default:
			if v.ClientEval {
				resClient = append(resClient, v)
				continue
			}

			res = append(res, v)
		}
	}

	return res, resClient
}

func And(ops ...Expr) Expr {
	res, resClient := simplifyAnd(ops...)

	if len(res) == 0 && len(resClient) == 0 {
		return True
	}

	if len(res) == 1 && len(resClient) == 0 {
		return res[0]
	}

	if len(resClient) == 1 && len(res) == 0 {
		return resClient[0]
	}

	return Expr{Type: AndOp, List: res, ListClient: resClient}
}

func simplifyOr(ops ...Expr) ([]Expr, []Expr) {
	var (
		res       = make([]Expr, 0, len(ops))
		resClient = make([]Expr, 0, len(ops))
	)

	for _, v := range ops {
		switch {
		case IsFalse(v):
			continue
		case IsTrue(v):
			return []Expr{True}, nil
		case v.Type == OrOp:
			r, c := simplifyOr(v.List...)
			if len(r) > 0 {
				res = append(res, r...)
			}

			if len(c) > 0 {
				resClient = append(resClient, c...)
			}

			if len(v.ListClient) > 0 {
				resClient = append(resClient, v.ListClient...)
			}
		default:
			if v.ClientEval {
				resClient = append(resClient, v)
				continue
			}

			res = append(res, v)
		}
	}

	return res, resClient
}

func Or(ops ...Expr) Expr {
	res, resClient := simplifyOr(ops...)

	if len(res) == 0 && len(resClient) == 0 {
		return False
	}

	if len(res) == 1 && len(resClient) == 0 {
		if IsTrue(res[0]) {
			return True
		}

		return res[0]
	}

	if len(resClient) == 1 && len(res) == 0 {
		if IsTrue(resClient[0]) {
			return True
		}

		return resClient[0]
	}

	return Expr{Type: OrOp, List: res, ListClient: resClient}
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

		lc := e.ListClient
		e.ListClient = make([]Expr, len(e.ListClient))

		for k := range lc {
			e.ListClient[k] = Negate(lc[k])
		}

		e.Type = OrOp
	case OrOp:
		l := e.List
		e.List = make([]Expr, len(e.List))

		for k := range l {
			e.List[k] = Negate(l[k])
		}

		lc := e.ListClient
		e.ListClient = make([]Expr, len(e.ListClient))

		for k := range lc {
			e.ListClient[k] = Negate(lc[k])
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

func (e Expr) Client() Expr {
	e.ClientEval = true
	return e
}
