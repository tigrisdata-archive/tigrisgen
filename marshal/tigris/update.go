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
	"sort"

	"github.com/tigrisdata/tigris-client-go/tigrisgen/expr"
	"github.com/tigrisdata/tigris-client-go/tigrisgen/util"
)

func marshalUpdateTempl(flt expr.Expr, buf *bytes.Buffer) string {
	buf.WriteString(string(expr.TemplOps[flt.Type]))
	buf.WriteString(" ")
	buf.WriteString(flt.X.Value.(string))
	buf.WriteString(" ")
	buf.WriteString("{{")
	buf.WriteString(flt.Y.Value.(string))
	buf.WriteString("}}")

	return buf.String()
}

func marshalUpdateTemplCond(flt expr.Expr) string {
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

func marshalUpdateExpr(upd expr.Expr, buf *bytes.Buffer) {
	n := util.Must(json.Marshal(upd.X.Value))
	v := util.Must(json.Marshal(upd.Y.Value))

	buf.Write(n)
	buf.WriteString(`:`)
	if upd.Y.Type == expr.Arg {
		buf.WriteString("{{.")
		buf.WriteString(upd.Y.Value.(string))
		buf.WriteString("}}")
	} else {
		buf.Write(v)
	}
}

func marshalUpdateLow(upd []expr.Expr, buf *bytes.Buffer) {
	group := make(map[expr.Op][]expr.Expr)

	// group by op type
	for _, v := range upd {
		/*
			if v.Type == expr.AndOp {
				for kk, vv := range v.List {

				}
			}
		*/

		group[v.Type] = append(group[v.Type], v)
	}

	keys := make([]string, 0)
	for k := range group {
		keys = append(keys, string(k))
	}

	sort.Strings(keys)

	buf.WriteString(`{`)
	i := 0
	for _, k := range keys {
		ops := group[expr.Op(k)]
		if i > 0 {
			buf.WriteString(`,`)
		}

		buf.WriteString(`"`)
		buf.WriteString(k)
		buf.WriteString(`":{`)

		for kk, vv := range ops {
			if kk > 0 {
				buf.WriteString(",")
			}

			marshalUpdateExpr(vv, buf)
		}
		buf.WriteString(`}`)

		i++
	}
	buf.WriteString(`}`)
}

func MarshalUpdate(upd []expr.Expr) string {
	var buf bytes.Buffer

	marshalUpdateLow(upd, &buf)

	return buf.String()
}
