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

package tigris

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tigrisdata/tigris-client-go/tigrisgen/expr"
)

func TestMarshalFilter(t *testing.T) {
	cases := []struct {
		name string
		flt  expr.Expr
		exp  string
	}{
		{name: "one", flt: expr.NewExpr(expr.Eq,
			expr.NewField("field1"),
			expr.NewConstant("value1"),
		), exp: `{"field1":"value1"}`},
		{name: "and", flt: expr.And(
			expr.NewExpr(expr.Eq, expr.NewField("field1"), expr.NewConstant("value1")),
			expr.NewExpr(expr.Eq, expr.NewField("field2"), expr.NewConstant("value2")),
		), exp: `{"$and":[{"field1":"value1"},{"field2":"value2"}]}`},
		{name: "or", flt: expr.Or(
			expr.NewExpr(expr.Eq, expr.NewField("field1"), expr.NewConstant("value1")),
			expr.NewExpr(expr.Eq, expr.NewField("field2"), expr.NewConstant("value2")),
		), exp: `{"$or":[{"field1":"value1"},{"field2":"value2"}]}`},
		{name: "and_or", flt: expr.And(
			expr.NewExpr(expr.Eq, expr.NewField("field1"), expr.NewConstant("value1")),
			expr.NewExpr(expr.Eq, expr.NewField("field2"), expr.NewConstant("value2")),
			expr.Or(
				expr.NewExpr(expr.Eq, expr.NewField("or_field1"), expr.NewConstant("or_value1")),
				expr.NewExpr(expr.Eq, expr.NewField("or_field2"), expr.NewConstant("or_value2")),
			),
		), exp: `{"$and":[{"field1":"value1"},{"field2":"value2"},{"$or":[{"or_field1":"or_value1"},{"or_field2":"or_value2"}]}]}`},
		{name: "or_and", flt: expr.Or(
			expr.NewExpr(expr.Eq, expr.NewField("field1"), expr.NewConstant("value1")),
			expr.NewExpr(expr.Eq, expr.NewField("field2"), expr.NewConstant("value2")),
			expr.And(
				expr.NewExpr(expr.Eq, expr.NewField("and_field1"), expr.NewConstant("and_value1")),
				expr.NewExpr(expr.Eq, expr.NewField("and_field2"), expr.NewConstant("and_value2")),
			),
		), exp: `{"$or":[{"field1":"value1"},{"field2":"value2"},{"$and":[{"and_field1":"and_value1"},{"and_field2":"and_value2"}]}]}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := MarshalFilter(c.flt)
			assert.Equal(t, c.exp, res)
		})
	}
}

func TestMarshalUpdate(t *testing.T) {
	cases := []struct {
		name string
		upd  []expr.Expr
		exp  string
	}{
		{name: "one", upd: []expr.Expr{
			expr.NewExpr(expr.SetOp, expr.NewField("field1"), expr.NewConstant(10))},
			exp: `{$set:{{"field1":10}}}`,
		},
		{name: "two", upd: []expr.Expr{
			expr.NewExpr(expr.SetOp, expr.NewField("field1"), expr.NewConstant(10)),
			expr.NewExpr(expr.IncOp, expr.NewField("field2"), expr.NewArg("arg1"))},
			exp: `{$set:{{"field1":10}}},{$increment:{{"field2":{{.arg1}}}}}`,
		},
		{name: "many", upd: []expr.Expr{
			expr.NewExpr(expr.SetOp, expr.NewField("field1"), expr.NewConstant(10)),
			expr.NewExpr(expr.IncOp, expr.NewField("field2"), expr.NewArg("arg1")),
			expr.NewExpr(expr.IncOp, expr.NewField("field2.subField"), expr.NewArg("arg2")),
			expr.NewExpr(expr.SetOp, expr.NewField("field1.subField"), expr.NewConstant(20))},
			exp: `{$set:{{"field1":10},{"field1.subField":20}}},{$increment:{{"field2":{{.arg1}}},{"field2.subField":{{.arg2}}}}}`,
		},
	}

	for _, c := range cases {
		res := MarshalUpdate(c.upd)
		assert.Equal(t, c.exp, res)
	}
}
