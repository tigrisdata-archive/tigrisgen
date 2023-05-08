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

	"github.com/tigrisdata/tigris-client-go/tigrisgen/expr"

	"github.com/tigrisdata/tigris-client-go/tigrisgen/util"
)

func marshalTempl(flt expr.Expr, buf *bytes.Buffer) string {
	buf.WriteString(string(expr.TemplOps[flt.Type]))
	buf.WriteString(" ")
	buf.WriteString(flt.X.Value.(string))
	buf.WriteString(" ")
	buf.WriteString("{{")
	buf.WriteString(flt.Y.Value.(string))
	buf.WriteString("}}")

	return buf.String()
}

func marshalTemplCond(flt expr.Expr) string {
	b := util.Must(json.Marshal(flt.Y))

	var buf bytes.Buffer
	buf.WriteString("{{if ")
	buf.WriteString(string(expr.TemplOps[flt.Type]))
	buf.WriteString(" ")
	buf.WriteString(flt.X.Value.(string))
	buf.WriteString(" ")
	buf.WriteString(string(b))
	buf.WriteString("}}")

	return buf.String()

}

func marshalCond(flt expr.Expr, buf *bytes.Buffer) {
	n := util.Must(json.Marshal(flt.X.Value))
	v := util.Must(json.Marshal(flt.Y.Value))

	buf.WriteString(`{`)
	buf.Write(n)
	buf.WriteString(`:`)
	if flt.Type == expr.Eq {
		if flt.Y.Type == expr.Arg {
			buf.WriteString("{{.")
			buf.WriteString(flt.Y.Value.(string))
			buf.WriteString("}}")
		} else {
			buf.Write(v)
		}
	} else {
		buf.WriteString(`{"`)
		buf.WriteString(string(flt.Type))
		buf.WriteString(`":`)
		if flt.Y.Type == expr.Arg {
			buf.WriteString("{{.")
			buf.WriteString(flt.Y.Value.(string))
			buf.WriteString("}}")
		} else {
			buf.Write(v)
		}
		buf.WriteString(`}`)
	}
	buf.WriteString(`}`)
}

func marshalFilterLow(flt expr.Expr, buf *bytes.Buffer) string {
	switch flt.Type {
	case expr.OrOp:
		buf.WriteString(`{"` + string(expr.OrOp) + `":[`)
		for i, vv := range flt.List {
			if i > 0 {
				buf.WriteString(",")
			}
			_ = marshalFilterLow(vv, buf)
		}
		buf.WriteString("]}")
	case expr.AndOp:
		buf.WriteString(`{"` + string(expr.AndOp) + `":[`)

		cnt := 0
		var clientBuf bytes.Buffer
		var andBuf bytes.Buffer
		for i, vv := range flt.List {
			if i > 0 {
				andBuf.WriteString(",")
			}
			if clientSide := marshalFilterLow(vv, &andBuf); clientSide != "" {
				clientBuf.WriteString(clientSide)
				cnt++
			}
		}
		if cnt > 0 {
			buf.Write(clientBuf.Bytes())
		}
		buf.Write(andBuf.Bytes())
		for ; cnt > 0; cnt-- {
			buf.WriteString("{{end}}")
		}

		buf.WriteString("]}")
	default:
		if flt.ClientEval {
			return marshalTemplCond(flt)
		} else {
			marshalCond(flt, buf)
		}
	}

	return ""
}

func MarshalFilter(flt expr.Expr) string {
	var buf bytes.Buffer

	if flt.Type == expr.TrueOp {
		return "{}"
	} else if flt.Type == expr.FalseOp {
		util.Fatal("filter always evaluates to false")
	}

	marshalFilterLow(flt, &buf)

	return buf.String()
}
