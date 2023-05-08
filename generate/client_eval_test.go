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

import "testing"

// Filter:
//
//	{"$or":[
//		{{ if ne .Arg.ArgInt 10 }}
//			{"field_float":{"$gt":100}},
//		{{end}}
//		{"field_float":{{toJSON .Arg.ArgFloat}}}
//	]}
func parseTestClientEval_and(d *Doc, args Args) bool {
	return args.ArgInt != 10 && d.FieldFloat > 100 || d.FieldFloat == args.ArgFloat
}

// Filter:
//
//	{"$or":[
//		{"field_float":{{toJSON .Arg.ArgFloat}}}
//		{{ if ne .Arg.ArgInt 10 }},
//			{"field_float":{"$gt":100}}
//		{{end}}
//	]}
func parseTestClientEval_and_arg_last(d *Doc, args Args) bool {
	return d.FieldFloat == args.ArgFloat || args.ArgInt != 10 && d.FieldFloat > 100
}

// Filter:
//
//	{"$or":[
//		{"field_float":{{toJSON .Arg.ArgFloat}}},
//		{{ if ne .Arg.ArgInt 10 }}
//			{"field_float":{"$gt":100}},
//		{{end}}
//		{"field_float":{{toJSON .Arg.NestedArg.ArgFloat}}}
//	]}
func parseTestClientEval_and_arg_middle(d *Doc, args Args) bool {
	return d.FieldFloat == args.ArgFloat || args.ArgInt != 10 && d.FieldFloat > 100 || d.FieldFloat == args.NestedArg.ArgFloat
}

// Filter:
//
//	{"$or":[
//		{{ if ne .Arg.ArgInt 10 }}
//			{"field_float":{"$gt":100}},
//		{{end}}
//		{"field_float":{{toJSON .Arg.NestedArg.ArgFloat}}}
//	]}
func parseTestClientEval_and_arg_first(d *Doc, args Args) bool {
	return args.ArgInt != 10 && d.FieldFloat > 100 || d.FieldFloat == args.NestedArg.ArgFloat
}

// Error:
//
//	Client side evaluated expressions are not allowed in the OR condition These are the expressions which doesn't include document fields
func parseTestClientEval_or(d *Doc, args Args) bool {
	return args.ArgInt != 10 || d.FieldFloat > 100
}

// Filter:
//
//	{{ if and ( ne .Arg.ArgInt 10 ) ( ne .Arg.ArgInt 11 ) }}
//		{"field_float":{{toJSON .Arg.ArgFloat}}}
//	{{end}}
func parseTestClientEval_if(d *Doc, args Args) bool {
	if args.ArgInt != 10 {
		return args.ArgInt != 11 && d.FieldFloat == args.ArgFloat
	}

	return false
}

// Filter:
//
//	{"$or":[
//		{{ if and ( eq .Arg.ArgBool true ) ( ne .Arg.ArgInt 10 ) }}
//			{"field_float":{{toJSON .Arg.ArgFloat}}},
//		{{end}}
//		{{ if and ( or ( ne .Arg.ArgBool true ) ( eq .Arg.ArgInt 10 ) ) ( eq .Arg.ArgInt 110 ) }}
//			{"field_float":{"$ne":{{toJSON .Arg.ArgFloat}}}}
//		{{end}}
//	]}
func parseTestClientEval_if_nested(d *Doc, args Args) bool {
	if args.ArgBool {
		if args.ArgInt != 10 {
			return d.FieldFloat == args.ArgFloat
		}
	}

	return args.ArgInt == 110 && d.FieldFloat != args.ArgFloat
}

// Filter:
//
//	{"$or":[
//		{{ if eq .Arg.ArgBool true }}
//			{"field_float":{{toJSON .Arg.ArgFloat}}},
//		{{end}}
//		{{ if and ( ne .Arg.ArgBool true ) ( eq .Arg.ArgInt 18 ) }}
//			{"field_string":"val1"},
//		{{end}}
//		{{ if and ( ne .Arg.ArgBool true ) ( ne .Arg.ArgInt 18 ) ( eq .Arg.ArgInt 110 ) }}
//			{"field_float":{"$ne":{{toJSON .Arg.ArgFloat}}}}
//		{{end}}
//	]}
func parseTestClientEval_if_else(d *Doc, args Args) bool {
	if args.ArgBool {
		return d.FieldFloat == args.ArgFloat
	} else if args.ArgInt == 18 {
		return d.FieldString == "val1"
	}

	return args.ArgInt == 110 && d.FieldFloat != args.ArgFloat
}

func TestFiltersClientEval(t *testing.T) {
	execTests(t, "parseTestClientEval_", false)
}

// Update:
//
//	{"$set":{
//		{{ if eq .Arg.ArgInt 10 }}
//			"field_int":10
//		{{end}}
//		{{ if eq .Arg.ArgInt 10 }},{{end}}
//		"field_float":1.1
//	}}
func parseTestUpdateClientEval_first(d *Doc, args Args) {
	if args.ArgInt == 10 {
		d.FieldInt = 10
	}

	d.FieldFloat = 1.1
}

// Update:
//
//	{"$set":{
//		"field_float":1.1
//		{{ if eq .Arg.ArgInt 10 }},
//		"field_int":10
//		{{end}}
//	}}
func parseTestUpdateClientEval_last(d *Doc, args Args) {
	d.FieldFloat = 1.1

	if args.ArgInt == 10 {
		d.FieldInt = 10
	}
}

// Update:
//
//	{"$set":{
//		"field_float":1.1
//		{{ if eq .Arg.ArgInt 10 }}
//		,"field_int":10
//		{{end}}
//		,"field_bool":true
//		}
//	}
func parseTestUpdateClientEval_middle(d *Doc, args Args) {
	d.FieldFloat = 1.1

	if args.ArgInt == 10 {
		d.FieldInt = 10
	}

	d.FieldBool = true
}

// Update:
//
//	{"$set":{
//		"field_float":1.1
//		{{ if eq .Arg.ArgString "qwerty" }}
//		,"field_string":"abc"
//		{{end}}
//		,"field_bool":true
//		}
//	}
func parseTestUpdateClientEval_string(d *Doc, args Args) {
	d.FieldFloat = 1.1

	if args.ArgString == "qwerty" {
		d.FieldString = "abc"
	}

	d.FieldBool = true
}

// Update:
//
//	{"$set":{
//		{{ if eq .Arg.ArgString "qwerty" }}"field_string":"abc"{{end}}
//		{{ if eq .Arg.ArgInt 10 }},"field_int":22{{end}}
//		{{ if eq .Arg.ArgFloat 3.3 }},"field_float":5.5{{end}}
//		{{ if or ( eq .Arg.ArgString "qwerty" ) ( eq .Arg.ArgInt 10 ) ( eq .Arg.ArgFloat 3.3 ) }},{{end}}
//		"field_bool":true
//	}}
func parseTestUpdateClientEval_multiple_optional(d *Doc, args Args) {
	if args.ArgString == "qwerty" {
		d.FieldString = "abc"
	}

	if args.ArgInt == 10 {
		d.FieldInt = 22
	}

	if args.ArgFloat == 3.3 {
		d.FieldFloat = 5.5
	}

	d.FieldBool = true
}

// Update:
//
//	{"$set":{
//		{{ if eq .Arg.ArgInt 10 }}
//			"field_int":22
//			{{ if eq .Arg.ArgString "qwerty" }},
//				{{ if eq .Arg.ArgFloat 3.3 }}
//					"field_float":5.5
//				{{end}}
//				{{ if eq .Arg.ArgFloat 3.3 }},{{end}}
//				"field_string":"abc"
//			{{end}}
//			,"field_string":"uuu"
//		{{end}}
//	}}
func parseTestUpdateClientEval_nested(d *Doc, args Args) {
	if args.ArgInt == 10 {
		d.FieldInt = 22

		if args.ArgString == "qwerty" {
			if args.ArgFloat == 3.3 {
				d.FieldFloat = 5.5
			}

			d.FieldString = "abc"
		}

		d.FieldString = "uuu"
	}
}

// Update:
//
//	{"$set":{
//		{{ if eq .Arg.ArgInt 10 }}
//			"field_int":22
//			{{ if eq .Arg.ArgString "qwerty" }},
//				{{ if eq .Arg.ArgFloat 3.3 }}
//					"field_float":5.5
//				{{end}}
//				{{ if eq .Arg.ArgFloat 3.3 }},{{end}}
//				"field_string":"abc"
//			{{end}},
//			"field_string":"uuu"
//		{{end}}
//	},
//	"$increment":{
//		{{ if eq .Arg.ArgInt 10 }}
//			"field_int":22
//		{{end}}
//	},
//	"$divide":{
//		{{ if eq .Arg.ArgInt 10 }}
//			{{ if eq .Arg.ArgString "qwerty" }}
//				"nested.field_int":888
//			{{end}}
//		{{end}}
//	},
//	"$multiply":{
//		{{ if eq .Arg.ArgInt 10 }}
//			"nested.field_float":777
//		{{end}}
//	},
//	"$push":{
//		{{ if eq .Arg.ArgInt 10 }}
//			{{ if eq .Arg.ArgString "qwerty" }}
//				{{ if eq .Arg.ArgFloat 3.3 }}
//					"field_arr_float":5.5
//				{{end}}
//			{{end}}
//		{{end}}
//	}}
func parseTestUpdateClientEval_multiop(d *Doc, args Args) {
	if args.ArgInt == 10 {
		d.FieldInt = 22
		d.FieldInt += 22

		if args.ArgString == "qwerty" {
			if args.ArgFloat == 3.3 {
				d.FieldFloat = 5.5
				d.FieldArrFloat = append(d.FieldArrFloat, 5.5)
			}

			d.FieldString = "abc"
			d.Nested.FieldInt /= 888
		}

		d.FieldString = "uuu"
		d.Nested.FieldFloat *= 777
	}
}

// this is to fix the unused linter.
var (
	_ = parseTestClientEval_and
	_ = parseTestClientEval_and_arg_last
	_ = parseTestClientEval_and_arg_middle
	_ = parseTestClientEval_and_arg_first
	_ = parseTestClientEval_or
	_ = parseTestClientEval_if
	_ = parseTestClientEval_if_nested
	_ = parseTestClientEval_if_else
	_ = parseTestUpdateClientEval_first
	_ = parseTestUpdateClientEval_last
	_ = parseTestUpdateClientEval_middle
	_ = parseTestUpdateClientEval_string
	_ = parseTestUpdateClientEval_multiple_optional
	_ = parseTestUpdateClientEval_nested
	_ = parseTestUpdateClientEval_multiop
)

func TestUpdateClientEval(t *testing.T) {
	execTests(t, "parseTestUpdateClientEval_", true)
}
