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
	"strings"

	"github.com/tigrisdata/tigrisgen/expr"
	"github.com/tigrisdata/tigrisgen/util"
)

func marshalUpdateExpr(upd expr.Expr, buf *bytes.Buffer) {
	n := util.Must(json.Marshal(upd.X.Value))
	v := util.Must(json.Marshal(upd.Y.Value))

	buf.Write(n)
	buf.WriteString(`:`)

	if upd.Y.Type == expr.Arg {
		if s, ok := upd.Y.Value.(string); ok && strings.HasPrefix(s, "{{") {
			buf.WriteString(s)
			return
		}

		buf.WriteString("{{toJSON .Arg")

		if upd.Y.Value.(string) != "" {
			buf.WriteString(".")
			buf.WriteString(upd.Y.Value.(string))
		}

		buf.WriteString("}}")
	} else {
		buf.Write(v)
	}
}

func marshalTmplExpr(flt expr.Expr, buf *bytes.Buffer) {
	if flt.Type == expr.OrOp || flt.Type == expr.AndOp {
		if len(flt.ListClient) > 1 {
			buf.WriteString(string(expr.TemplOps[flt.Type]))

			for _, vv := range flt.ListClient {
				buf.WriteString(" ")
				buf.WriteString("( ")
				marshalTmplExpr(vv, buf)
				buf.WriteString(" )")
			}
		} else {
			marshalTmplCondLow(flt.ListClient[0], buf)
		}
	} else {
		marshalTmplCondLow(flt, buf)
	}
}

func marshalCommaCond(conds []expr.Expr, buf *bytes.Buffer) {
	buf.WriteString("{{ if ")

	if len(conds) > 1 {
		buf.WriteString(string(expr.TemplOps[expr.OrOp]))

		for _, v := range conds {
			buf.WriteString(" ")
			buf.WriteString("( ")
			marshalTmplExpr(v, buf)
			buf.WriteString(" )")
		}
	} else {
		marshalTmplExpr(conds[0], buf)
	}

	buf.WriteString(" }}")
}

func marshalUpdateOp(op expr.Op, upd []expr.Expr, buf *bytes.Buffer) {
	i := 0
	firstNonOptional := true

	var nonEmptyConds []expr.Expr

	for _, vv := range upd {
		if vv.Type == op {
			if i > 0 {
				if firstNonOptional {
					marshalCommaCond(nonEmptyConds, buf)
					buf.WriteString(`,`)
					buf.WriteString(`{{end}}`)
				} else {
					buf.WriteString(`,`)
				}
			}

			marshalUpdateExpr(vv, buf)

			i++

			firstNonOptional = false
		} else if vv.Type == expr.UpdIfOp {
			var opBuf bytes.Buffer

			marshalUpdateOp(op, vv.List, &opBuf)

			if opBuf.Len() > 0 {
				if firstNonOptional {
					nonEmptyConds = append(nonEmptyConds, vv.ListClient[0])
				}

				buf.WriteString(`{{ if `)
				marshalTmplExpr(vv.ListClient[0], buf)
				buf.WriteString(` }}`)

				if i > 0 {
					buf.WriteString(`,`)
				}

				buf.Write(opBuf.Bytes())

				buf.WriteString(`{{end}}`)
				i++
			}
		}
	}
}

func marshalUpdateLow(upd []expr.Expr, buf *bytes.Buffer) {
	buf.WriteString(`{`)

	prev := false

	for _, v := range expr.UpdOps {
		var opBuf bytes.Buffer

		marshalUpdateOp(v, upd, &opBuf)

		if opBuf.Len() > 0 {
			if prev {
				buf.WriteString(`,`)
			}

			buf.WriteString(`"`)
			buf.WriteString(string(v))
			buf.WriteString(`":{`)

			buf.Write(opBuf.Bytes())

			buf.WriteString(`}`)

			prev = true
		}
	}

	buf.WriteString(`}`)
}

func MarshalUpdate(upd []expr.Expr) string {
	var buf bytes.Buffer

	marshalUpdateLow(upd, &buf)

	return buf.String()
}
