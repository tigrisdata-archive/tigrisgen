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
	"bytes"
	"encoding/json"

	"github.com/tigrisdata/tigrisgen/expr"
	"github.com/tigrisdata/tigrisgen/util"
)

func marshalTmplCondLow(flt expr.Expr, buf *bytes.Buffer) {
	b := util.Must(json.Marshal(flt.Y.Value))

	buf.WriteString(string(expr.TemplOps[flt.Type]))
	buf.WriteString(" .Arg")

	if flt.X.Value.(string) != "" {
		buf.WriteString(".")
		buf.WriteString(flt.X.Value.(string))
	}

	buf.WriteString(" ")
	buf.WriteString(string(b))
}

func marshalTmplCond(flt expr.Expr, buf *bytes.Buffer) {
	b := util.Must(json.Marshal(flt.Y.Value))

	buf.WriteString("{{ if ")
	buf.WriteString(string(expr.TemplOps[flt.Type]))
	buf.WriteString(" .Arg")

	if flt.X.Value.(string) != "" {
		buf.WriteString(".")
		buf.WriteString(flt.X.Value.(string))
	}

	buf.WriteString(" ")
	buf.WriteString(string(b))
	buf.WriteString(" }}")
}

func marshalListTmplCond(flt expr.Expr, buf *bytes.Buffer) {
	buf.WriteString("{{ if ")
	buf.WriteString(string(expr.TemplOps[flt.Type]))
	buf.WriteString(" ")

	for _, v := range flt.ListClient {
		if v.Type == expr.OrOp {
			buf.WriteString("( ")
			buf.WriteString(string(expr.TemplOps[v.Type]))

			for _, vv := range v.ListClient {
				buf.WriteString(" ")
				buf.WriteString("( ")
				marshalTmplCondLow(vv, buf)
				buf.WriteString(" )")
			}

			buf.WriteString(" )")
		} else {
			buf.WriteString(" ")
			buf.WriteString("( ")
			marshalTmplCondLow(v, buf)
			buf.WriteString(" )")
		}
	}

	buf.WriteString(" }}")
}

func marshalCond(flt expr.Expr, buf *bytes.Buffer) {
	n := util.Must(json.Marshal(flt.X.Value))
	v := util.Must(json.Marshal(flt.Y.Value))

	buf.WriteString(`{`)
	buf.Write(n)
	buf.WriteString(`:`)

	if flt.Type == expr.Eq {
		if flt.Y.Type == expr.Arg {
			buf.WriteString("{{toJSON .Arg")

			if flt.Y.Value.(string) != "" {
				buf.WriteString(".")
				buf.WriteString(flt.Y.Value.(string))
			}

			buf.WriteString("}}")
		} else {
			buf.Write(v)
		}
	} else {
		buf.WriteString(`{"`)
		buf.WriteString(string(flt.Type))
		buf.WriteString(`":`)
		if flt.Y.Type == expr.Arg {
			buf.WriteString("{{toJSON .Arg")
			if flt.Y.Value.(string) != "" {
				buf.WriteString(".")
				buf.WriteString(flt.Y.Value.(string))
			}
			buf.WriteString("}}")
		} else {
			buf.Write(v)
		}
		buf.WriteString(`}`)
	}

	buf.WriteString(`}`)
}

func marshalArray(flt expr.Expr, buf *bytes.Buffer) {
	buf.WriteString(`{"` + string(flt.Type) + `":[`)

	nOptional := 0

	for _, vv := range flt.List {
		if len(vv.ListClient) > 0 {
			nOptional++
		}
	}

	oneNonOpt := (len(flt.List) - nOptional) == 1

	preComma := false

	for i, vv := range flt.List {
		comma := i < len(flt.List)-1 && (!oneNonOpt || len(vv.ListClient) != 0)
		marshalFilterLow(vv, buf, preComma, comma)
		preComma = oneNonOpt && len(vv.ListClient) == 0
	}

	buf.WriteString("]}")
}

func putComma(need bool, buf *bytes.Buffer) {
	if need {
		buf.WriteString(",")
	}
}

func marshalFilterLow(flt expr.Expr, buf *bytes.Buffer, preComma, comma bool) {
	switch flt.Type {
	case expr.OrOp:
		if len(flt.ListClient) > 0 && len(flt.List) > 0 {
			util.Fatal("Client side evaluated expressions are not allowed in the OR condition\n" +
				"These are the expressions which doesn't include document fields")
		}

		if len(flt.ListClient) == 1 {
			marshalTmplCond(flt.ListClient[0], buf)
		} else if len(flt.ListClient) > 0 {
			marshalListTmplCond(flt, buf)
		} else if len(flt.List) == 1 {
			marshalFilterLow(flt.List[0], buf, false, false)
		} else {
			marshalArray(flt, buf)
		}

		putComma(comma, buf)
	case expr.AndOp:
		if len(flt.ListClient) == 1 {
			marshalTmplCond(flt.ListClient[0], buf)
		} else if len(flt.ListClient) > 1 {
			marshalListTmplCond(flt, buf)
		}

		putComma(preComma, buf)

		if len(flt.List) == 1 {
			marshalFilterLow(flt.List[0], buf, false, false)
		} else {
			marshalArray(flt, buf)
		}

		putComma(comma, buf)

		if len(flt.ListClient) > 0 {
			buf.WriteString("{{end}}")
		}
	default:
		marshalCond(flt, buf)

		putComma(comma, buf)
	}
}

func MarshalFilter(flt expr.Expr) string {
	var buf bytes.Buffer

	if flt.Type == expr.TrueOp {
		return "{}"
	} else if flt.Type == expr.FalseOp {
		util.Fatal("filter always evaluates to false")
	}

	marshalFilterLow(flt, &buf, false, false)

	return buf.String()
}
