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

package main

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/printer"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tigrisdata/tigris-client-go/tigris"

	"github.com/tigrisdata/tigris-client-go/tigrisgen/util"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/tigrisdata/tigris-client-go/test"
)

const (
	testConstInt    = 10 + 18
	testConstString = "aaa" + "bbb"
)

type Nested struct {
	FieldInt    int       `json:"field_int"`
	FieldFloat  float64   `json:"field_float"`
	FieldString string    `json:"field_string"`
	FieldBool   bool      `json:"field_bool"`
	FieldTime   time.Time `json:"field_time"`
	FieldUUID   uuid.UUID `json:"field_uuid"`
	FieldBytes  []byte    `json:"field_bytes"`
	FieldArr    []Nested  `json:"field_arr"`

	FieldArrFloat []float64 `json:"field_arr_float"`
}

type NestedArg struct {
	ArgInt    int
	ArgFloat  float64
	ArgString string
	ArgBool   bool
	ArgTime   time.Time
	ArgUUID   uuid.UUID
	ArgBytes  []byte
}

type Doc struct {
	FieldInt    int       `json:"field_int"`
	FieldFloat  float64   `json:"field_float"`
	FieldString string    `json:"field_string"`
	FieldBool   bool      `json:"field_bool"`
	FieldTime   time.Time `json:"field_time"`
	FieldUUID   uuid.UUID `json:"field_uuid"`
	FieldBytes  []byte    `json:"field_bytes"`
	FieldArr    []Nested  `json:"field_arr"`

	FieldArrFloat []float64 `json:"field_arr_float"`

	Nested Nested `json:"nested"`
}

type Args struct {
	ArgInt    int
	ArgFloat  float64
	ArgString string
	ArgBool   bool
	ArgTime   time.Time
	ArgUUID   uuid.UUID
	ArgBytes  []byte

	NestedArg NestedArg
}

func FilterOne(d Doc, i float64) bool {
	return d.FieldInt != 10 && d.FieldFloat > 100 || d.FieldFloat == i
}

func FilterOnePtr(d *Doc, i float64) bool {
	return d.FieldInt != 10 && d.FieldFloat > 100
}

func (d Doc) FilterOne(args float64) bool {
	return d.FieldInt != 10 && d.FieldFloat > 122
}

func (d *Doc) FilterOnePtr(args float64) bool {
	return d.FieldInt != 10 && d.FieldFloat > 100
}

func UpdateOne(d Doc, a Args) {
	d.FieldInt += a.ArgInt
}

func UpdateOneAll(d Doc, a Args) {
	d.FieldInt += a.ArgInt
	d.FieldInt -= a.ArgInt
	d.FieldInt *= a.ArgInt
	d.FieldInt /= a.ArgInt
	d.FieldInt = a.ArgInt

	d.FieldInt += 10
	d.FieldInt -= 10
	d.FieldInt *= 10
	d.FieldInt /= 10
	d.FieldInt = 10

	d.FieldFloat *= 10
	d.FieldFloat = 10
	d.FieldString = "abc"
	d.FieldBool = true

	d.Nested.FieldString = "gh"
	d.FieldArr[1].FieldString = "gh"
	d.FieldArrFloat[23] = 1.2
	d.Nested.FieldArr[7].FieldBool = true

	d.FieldUUID = a.ArgUUID
	d.FieldTime = a.ArgTime
	d.FieldBytes = a.ArgBytes
}

/*
func UpdateOnePtr(d *Doc, i int) {
	d.FieldInt += i
}
*/

func (d Doc) UpdateOne(args int) {
	d.FieldInt += args
}

func (d *Doc) UpdateOnePtr(args int) {
	d.FieldInt += args
}

func TestAPILookup(t *testing.T) {
	s, _ := os.Getwd()
	log.Debug().Strs("args", os.Args).Str("pwd", s).Msg("Starting")

	p := loadProgram([]string{"."})

	for _, pi := range p.InitialPackages() {
		f, u := findAndParse(p, pi)

		require.Equal(t, 5, len(f))
		require.Equal(t, 5, len(u))

		for _, v := range f {
			switch v.Name {
			case "main.FilterOne":
				require.Equal(t, `{"$or":[{"$and":[{"FieldInt":{"$ne":10}},{"FieldFloat":{"$gt":100}}]},{"FieldFloat":{"$eq":"{{.}}"}}]}`,
					v.Body)
			case "github.com/tigrisdata/tigris-client-go/test.FilterOne":
				require.Equal(t, `{"$and":[{"Field1":{"$ne":10}},{"Field2":{"$gt":99}}]}`,
					v.Body)
			case "main.Doc.FilterOne":
				require.Equal(t, `{"$and":[{"FieldInt":{"$ne":10}},{"FieldFloat":{"$gt":122}}]}`,
					v.Body)
			case "github.com/tigrisdata/tigris-client-go/test.Doc.FilterOne":
				require.Equal(t, `{"$and":[{"Field1":{"$ne":10}},{"Field2":{"$gt":111}}]}`,
					v.Body)
			}
		}

		for _, v := range u {
			switch v.Name {
			case "main.UpdateOne":
				require.Equal(t, `{"$increment":{"FieldInt":"{{.ArgInt}}"}}`, v.Body)
			case "github.com/tigrisdata/tigris-client-go/test.UpdateOne":
				require.Equal(t, `{"$increment":{"Field1":"{{.}}"}}`, v.Body)
			case "main.Doc.UpdateOne":
				require.Equal(t, `{"$increment":{"FieldInt":"{{.}}"}}`, v.Body)
			case "github.com/tigrisdata/tigris-client-go/test.Doc.UpdateOne":
				require.Equal(t, `{"$increment":{"Field1":"{{.}}","Field2":10},"$decrement":{"Field2":10}}`,
					v.Body)
			}
		}
	}
}

// These Tigris API calls are detected in the above TestAPILookup
func UpdateAPICalls() {
	ctx := context.TODO()

	db, err := tigris.OpenDatabase(ctx, &tigris.Config{URL: "localhost:8081"})
	if err != nil {
		panic(err)
	}

	c := tigris.GetCollection[test.Doc](db)
	c1 := tigris.GetCollection[Doc](db)

	_, _ = tigris.Update(ctx, c, test.FilterOne, test.UpdateOne, 1.23, 10)
	_, _ = tigris.Update(ctx, c1, FilterOne, UpdateOne, 1.24, Args{})

	_, _ = tigris.Update(ctx, c1, Doc.FilterOne, Doc.UpdateOne, 1.24, 10)
	_, _ = tigris.Update(ctx, c, test.Doc.FilterOne, test.Doc.UpdateOne, 1.23, 10)

	_, _ = tigris.Update[Doc, Args, int](ctx, c1,
		func(d Doc, a Args) bool {
			return d.FieldInt < a.ArgInt
		},
		func(d Doc, a int) {
			d.FieldInt = a
		},
		Args{ArgInt: 10}, 10)
}

// Filter: {"Field2":{"$lt":10}}
func parseTest_simple(d *test.Doc, _ Args) bool {
	return d.Field2 < 10
}

// Filter: {"nested.field_222":{"$lt":{{.ArgString}}}}
func parseTest_nested(d *test.Doc, args Args) bool {
	return d.Nested.Field222 < args.ArgString
}

// Filter: {"$or":[{"$and":[{"field_int":{"$lt":20}},{"field_float":-8.4}]},{"field_string":"aaabbb"},{"field_int":28}]}
func parseTest_calculated_constants(d *Doc, _ Args) bool {
	return d.FieldInt < 10+10 && d.FieldFloat == 10.3-18.7 ||
		d.FieldString == testConstString || d.FieldInt == testConstInt
}

// Filter: {"$and":[{"$or":[{"$and":[{"Field1":{"$lt":10}},{"Field3":10.1}]},{"Field1":{{.ArgInt}}},{"$and":[{"Field2":{"$gt":15}},{"Field2":{"$lt":10}}]}]},{"Field3":{"$lt":18}}]}
func parseTest_logical_expression(d *test.Doc, args Args) bool {
	return (d.Field1 < 10 && d.Field3 == 10.1 || d.Field1 == args.ArgInt ||
		(d.Field2 > 15 && d.Field2 < 10)) && d.Field3 < 18
}

// Filter: {"$or":[{"Field1":{"$lte":20}},{"Field1":{"$gte":{{.ArgInt}}}}]}
func parseTest_field_order(d *test.Doc, args Args) bool {
	return 10+10 > d.Field1 || args.ArgInt < d.Field1
}

// Filter: {"Field3":{"$lte":{{.}}}}
func parseTest_simple_arg(d *test.Doc, f float64) bool {
	return f > d.Field3
}

// Filter: {"Field1":{"$lt":{{.NestedArg.ArgInt}}}}
func parseTest_nested_arg(d *test.Doc, args Args) bool {
	return d.Field1 < args.NestedArg.ArgInt
}

// Filter: {"Field1":{"$lt":10}}
func parseTest_notag(d *test.Doc, args Args) bool {
	return d.Field1 < 10
}

// Filter: {"$or":[{"field_bool":true},{"nested.field_bool":{"$ne":true}}]}
func parseTest_bool(d *Doc, args Args) bool {
	return d.FieldBool || !d.Nested.FieldBool
}

// Filter: {"$or":[{"field_arr.1.field_bool":true},{"nested.field_arr_float.5":{{.ArgFloat}}}]}
func parseTest_arrays(d *Doc, args Args) bool {
	return d.FieldArr[1].FieldBool || d.Nested.FieldArrFloat[5] == args.ArgFloat
}

// Filter: {"$or":[{"field_arr.{{.ArgInt}}.field_bool":true},{"nested.field_arr_float.5":{{.ArgFloat}}}]}
func parseTest_arrays_arg(d *Doc, args Args) bool {
	return d.FieldArr[args.ArgInt].FieldBool || d.Nested.FieldArrFloat[5] == args.ArgFloat
}

// Filter: {"$or":[{"field_bool":true},{"field_bytes":{"$lte":{{.ArgBytes}}}},{"field_string":{"$contains":{{.ArgString}}}},{"field_string":{"$not_contains":{{.ArgString}}}}]}
func parseTest_func(d *Doc, args Args) bool {
	return d.FieldBool || bytes.Compare(args.ArgBytes, d.FieldBytes) > 0 || strings.Contains(d.FieldString, args.ArgString) ||
		!strings.Contains(d.FieldString, args.ArgString)
}

// Filter: {}
func parseTest_or_true(d *Doc, _ Args) bool {
	return d.FieldInt == 1 || true
}

// Filter: {"field_int":1}
func parseTest_or_true_nested(d *Doc, _ Args) bool {
	return d.FieldInt == 1 && (true || d.FieldInt != 1 || false)
}

func cleanupComment(comment string) string {
	comment = strings.Trim(strings.TrimPrefix(comment, "Filter:"), " ")
	comment = strings.Trim(strings.TrimPrefix(comment, "Update:"), " ")
	comment = strings.Trim(strings.TrimPrefix(comment, "Error:"), " ")
	comment = strings.ReplaceAll(comment, "\n", "")
	comment = strings.ReplaceAll(comment, "\t", "")
	comment = strings.ReplaceAll(comment, " ", "")

	return comment
}

func validateTestComment(t *testing.T, fn *ast.FuncDecl, update bool) {
	if fn.Doc == nil ||
		(update && !strings.HasPrefix(fn.Doc.Text(), "Update:") ||
			!update && !strings.HasPrefix(fn.Doc.Text(), "Filter:")) &&
			!strings.HasPrefix(fn.Doc.Text(), "Error:") {
		t.Fatal(`filter test should contain a comment in the format:
// Filter: {expected result filter in JSON+gotmpl format}

or

// Update: {expected result filter in JSON+gotmpl format}

or

// Error: {expected error}
`)
	}
}

func setupFatalHandlers() {
	FatalWithExpr = func(e ast.Node, format string, args ...any) {
		cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}
		var buf bytes.Buffer
		_ = cfg.Fprint(&buf, Program.Fset, e)
		panic(fmt.Sprintf("%s: %s", fmt.Sprintf(format, args...), buf.String()))
	}

	util.Fatal = func(format string, args ...any) {
		panic(fmt.Sprintf(format, args...))
	}
}

func catchFatalError(errMsg *string) {
	res := recover()
	if res != nil {
		s, ok := res.(string)
		if !ok {
			panic(res)
		}
		*errMsg = s
	}
}

func execTests(t *testing.T, prefix string, update bool) {
	p := Program

	setupFatalHandlers()

	require.Equal(t, 1, len(p.InitialPackages()))
	pi := p.InitialPackages()[0]

	for _, f := range pi.Files {
		for _, v := range f.Decls {
			fn, ok := v.(*ast.FuncDecl)
			if !ok {
				continue
			}

			if !strings.HasPrefix(fn.Name.Name, prefix) {
				continue
			}

			t.Run(fn.Name.Name, func(t *testing.T) {
				log.Debug().Str("file", p.Fset.Position(fn.Pos()).String()).
					Str("function", fn.Name.Name).Msg("test parsing filter")

				var flt string
				var errMsg string

				func() {
					defer catchFatalError(&errMsg)

					if update {
						_, flt = parseUpdateFunction(fn.Name.Name, fn, pi)
					} else {
						_, flt = parseFilterFunction(fn.Name.Name, fn, pi)
					}
				}()

				validateTestComment(t, fn, update)

				testError := strings.HasPrefix(fn.Doc.Text(), "Error:")

				comment := cleanupComment(fn.Doc.Text())

				if testError {
					assert.Equal(t, comment, strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(errMsg, " ", ""), "\n", ""), "\t", ""))
				} else {
					if errMsg != "" {
						assert.NoError(t, fmt.Errorf("unexpected error: %v", errMsg))
					} else {
						assert.Equal(t, comment, flt)
					}
				}
			})
		}
	}
}

func TestLogicalFilters(t *testing.T) {
	execTests(t, "parseTest_", false)
}

// Filter: {"$or":[{"field_int":10},{"field_float":{{.ArgFloat}}}]}
func parseFlowTest_1(d *Doc, args Args) bool {
	if d.FieldInt == 10 || d.FieldFloat == args.ArgFloat {
		return true
	}

	return false
}

// Filter:
//
//	{"$or":[
//		{"$and":[
//			{"field_float":{{.ArgFloat}}},
//			{"field_int":10}
//		]},
//		{"$and":[
//			{"field_float":{"$ne":{{.ArgFloat}}}},
//			{"field_bool":true}
//		]}
//	]}
func parseFlowTest_2(d *Doc, args Args) bool {
	if d.FieldFloat == args.ArgFloat {
		return d.FieldInt == 10
	}

	return d.FieldBool
}

// Filter:
//
//	{"$or":[{
//		"$and":[
//			{"field_float":{{.ArgFloat}}},
//			{"field_int":10}
//		]},
//		{"$and":[
//			{"field_float":{"$ne":{{.ArgFloat}}}},
//			{"$or":[
//				{"$and":[
//					{"field_float":{"$ne":{{.ArgFloat}}}},
//					{"field_bool":{"$ne":true}}
//				]},
//				{"$and":[
//					{"field_float":{{.ArgFloat}}},
//					{"field_int":{"$ne":22}}
//				]}
//			]}
//		]}
//	]}
func parseFlowTest_3(d *Doc, args Args) bool {
	if d.FieldFloat == args.ArgFloat {
		return d.FieldInt == 10
	}

	if d.FieldFloat != args.ArgFloat {
		return !d.FieldBool
	}

	return d.FieldInt != 22
}

// Filter:
//
//	{"$or":[
//		{"field_float":{{.ArgFloat}}},
//		{"$and":[
//			{"field_float":{"$ne":{{.ArgFloat}}}},
//			{"field_int":{{.ArgInt}}},
//			{"$or":[
//				{"field_float":{"$ne":{{.ArgFloat}}}},
//				{"$and":[
//					{"field_float":{{.ArgFloat}}},
//					{"$or":[
//						{"field_int":25},
//						{"$and":[
//							{"field_int":{"$ne":25}},
//							{"field_int":{"$ne":32}},
//							{"field_int":{"$ne":55}},
//							{"field_int":22}
//						]}
//					]}
//				]}
//			]}
//		]}
//	]}
func parseFlowTest_4(d *Doc, args Args) bool {
	if d.FieldFloat == args.ArgFloat {
		return true
	}

	if d.FieldInt != args.ArgInt {
		return false
	}

	if d.FieldFloat != args.ArgFloat {
		return true
	}

	if d.FieldInt == 25 {
		return true
	}

	if d.FieldInt == 32 {
		return false
	}

	if d.FieldInt == 55 {
		return false
	}

	return d.FieldInt == 22
}

// Filter:
//
//	{"$or":[
//		{"field_float":{{.ArgFloat}}},
//		{"$and":[
//			{"field_float":{"$ne":{{.ArgFloat}}}},
//			{"$or":[
//				{"$and":[
//					{"field_int":{"$ne":{{.ArgInt}}}},
//					{"$or":[
//						{"$and":[
//							{"field_bool":true},
//							{"nested.field_int":111}
//						]},
//						{"$and":[
//							{"field_bool":{"$ne":true}},
//							{"field_bool":{"$ne":true}},
//							{"nested.field_int":222}
//						]}
//					]}
//				]},
//				{"$and":[
//					{"field_int":{{.ArgInt}}},
//					{"field_float":{"$ne":{{.ArgFloat}}}}
//				]}
//			]}
//		]}
//	]}
func parseFlowTest_5(d *Doc, args Args) bool {
	if d.FieldFloat == args.ArgFloat {
		return true
	}

	if d.FieldInt != args.ArgInt {
		if d.FieldBool {
			return d.Nested.FieldInt == 111
		}
		if !d.FieldBool {
			return d.Nested.FieldInt == 222
		}
		return false
	}

	if d.FieldFloat != args.ArgFloat {
		return true
	}

	return false
}

// Filter:
//
//	{"$or":[
//		{"field_float":{{.ArgFloat}}},
//		{"$and":[
//			{"field_float":{"$ne":{{.ArgFloat}}}},
//			{"$or":[
//				{"$and":[
//					{"field_int":{"$ne":{{.ArgInt}}}},
//					{"field_bool":{"$ne":true}},
//					{"nested.field_int":222}
//				]},
//				{"$and":[
//					{"field_int":{{.ArgInt}}},
//					{"field_bool":{"$ne":true}},
//					{"nested.field_int":333}
//				]},
//				{"$and":[
//					{"field_int":{{.ArgInt}}},
//					{"field_bool":true},
//					{"field_float":{"$ne":{{.ArgFloat}}}}
//				]}
//			]}
//		]}
//	]}
func parseFlowTest_else_one_branch_fall_through(d *Doc, args Args) bool {
	if d.FieldFloat == args.ArgFloat {
		return true
	}

	if d.FieldInt != args.ArgInt {
		if !d.FieldBool {
			return d.Nested.FieldInt == 222
		}
		return false
	} else {
		if !d.FieldBool {
			return d.Nested.FieldInt == 333
		}
	}

	if d.FieldFloat != args.ArgFloat {
		return true
	}

	return false
}

// Filter:
//
//	{"$or":[
//		{"field_float":{{.ArgFloat}}},
//		{"$and":[
//			{"field_float":{"$ne":{{.ArgFloat}}}},
//			{"$or":[
//				{"$and":[
//					{"field_int":{"$ne":{{.ArgInt}}}},
//					{"field_bool":{"$ne":true}},
//					{"nested.field_int":222}
//				]},
//				{"$and":[
//					{"field_int":{{.ArgInt}}},
//					{"field_bool":{"$ne":true}},
//					{"nested.field_int":333}
//				]},
//				{"$and":[
//					{"$or":[
//						{"$and":[
//							{"field_int":{"$ne":{{.ArgInt}}}},
//							{"field_bool":true}
//						]},
//						{"$and":[
//							{"field_int":{{.ArgInt}}},
//							{"field_bool":true}
//						]}
//					]},
//					{"field_float":{"$ne":{{.ArgFloat}}}}
//				]}
//			]}
//		]}
//	]}
func parseFlowTest_else_two_branch_fallthrough(d *Doc, args Args) bool {
	if d.FieldFloat == args.ArgFloat {
		return true
	}

	if d.FieldInt != args.ArgInt {
		if !d.FieldBool {
			return d.Nested.FieldInt == 222
		}
	} else {
		if !d.FieldBool {
			return d.Nested.FieldInt == 333
		}
	}

	if d.FieldFloat != args.ArgFloat {
		return true
	}

	return false
}

// Filter:
//
//	{"$or":[
//		{"field_float":{{.ArgFloat}}},
//		{"$and":[
//			{"field_float":{"$ne":{{.ArgFloat}}}},
//			{"$or":[
//				{"$and":[
//					{"field_int":{"$ne":{{.ArgInt}}}},
//					{"$or":[
//						{"$and":[
//							{"field_bool":{"$ne":true}},
//							{"nested.field_int":222}
//						]},
//						{"$and":[
//							{"field_bool":true},
//							{"$or":[
//								{"$and":[
//									{"field_string":"aaaa"},
//									{"nested.field_int":444}
//								]},
//								{"field_string":{"$ne":"aaaa"}}
//							]}
//						]}
//					]}
//				]},
//				{"$and":[
//					{"field_int":{{.ArgInt}}},
//					{"$or":[
//						{"$and":[
//							{"field_bool":{"$ne":true}},
//							{"nested.field_int":333}
//						]},
//						{"$and":[
//							{"field_bool":true},
//							{"field_string":"bbbbb"},
//							{"nested.field_int":5555}
//						]}
//					]}
//				]},
//				{"$and":[
//					{"field_int":{{.ArgInt}}},
//					{"field_bool":true},
//					{"field_string":{"$ne":"bbbbb"}},
//					{"field_float":{{.ArgFloat}}}
//				]}
//			]}
//		]}
//	]}
func parseFlowTest_else_multi_if_both_branch(d *Doc, args Args) bool {
	if d.FieldFloat == args.ArgFloat {
		return true
	}

	if d.FieldInt != args.ArgInt {
		if !d.FieldBool {
			return d.Nested.FieldInt == 222
		}
		if d.FieldString == "aaaa" {
			return d.Nested.FieldInt == 444
		}
		return true
	} else {
		if !d.FieldBool {
			return d.Nested.FieldInt == 333
		}
		if d.FieldString == "bbbbb" {
			return d.Nested.FieldInt == 5555
		}
	}

	if d.FieldFloat != args.ArgFloat {
		return false
	}

	return true
}

// Filter:
//
//	{"$or":[
//		{"field_float":{{.ArgFloat}}},
//		{"$and":[
//			{"field_float":{"$ne":{{.ArgFloat}}}},
//			{"$or":[
//				{"$and":[
//					{"field_int":{"$ne":{{.ArgInt}}}},
//					{"field_string":",,,"},
//					{"nested.field_int":222}
//				]},
//				{"$and":[
//					{"field_int":{{.ArgInt}}},
//					{"field_bool":{"$ne":true}},
//					{"field_string":"bbbbb"},
//					{"nested.field_int":5555}
//				]},
//				{"$and":[
//					{"$or":[
//						{"$and":[
//							{"field_int":{"$ne":{{.ArgInt}}}},
//							{"field_string":{"$ne":",,,"}}
//						]},
//						{"$and":[
//							{"field_int":{{.ArgInt}}},
//							{"$or":[
//								{"field_bool":true},
//								{"field_string":{"$ne":"bbbbb"}}
//							]}
//						]}
//					]},
//					{"field_float":{{.ArgFloat}}}
//				]}
//			]}
//		]}
//	]}
func parseFlowTest_both_branch_fallthrough(d *Doc, args Args) bool {
	if d.FieldFloat == args.ArgFloat {
		return true
	}

	if d.FieldInt != args.ArgInt {
		if d.FieldString == ",,," {
			return d.Nested.FieldInt == 222
		}
	} else if !d.FieldBool {
		if d.FieldString == "bbbbb" {
			return d.Nested.FieldInt == 5555
		}
	}

	if d.FieldFloat != args.ArgFloat {
		return false
	}

	return true
}

// Filter: {}
func parseFlowTest_6(d *Doc, _ Args) bool {
	return true
}

// Filter: {"field_bool":true}
func parseFlowTest_8(d *Doc, _ Args) bool {
	return d.FieldBool
}

// Filter:
//
//	{"$and":[
//		{"field_int":1},
//		{"$or":[
//			{"field_float":15},
//			{"field_string":"ddd"}
//		]},
//		{"field_bool":true}
//	]}
func parseFlowTest_const_cond(d *Doc, _ Args) bool {
	if true {
		if d.FieldInt == 1 && (d.FieldFloat == 15 || d.FieldString == "ddd") {
			return d.FieldBool
		}
	}
	return false
}

// Filter: {"$and":[{"field_int":1},{"field_int":123}]}
func parseFlowTest_const_cond_1(d *Doc, _ Args) bool {
	if false {
		return d.FieldBool
	} else if true {
		if d.FieldInt == 1 {
			return d.FieldInt == 123
		}
	}
	return false
}

func TestFiltersControlFlow(t *testing.T) {
	execTests(t, "parseFlowTest_", false)
}

// Error: unreachable code: return d.FieldUUID==args.ArgUUID
func parseTestNegative_unreachable_1(d *Doc, args Args) bool {
	if d.FieldInt != args.ArgInt {
		return true
	} else {
		return false
	}

	return d.FieldUUID == args.ArgUUID
}

// Error:  unreachable code: return false
func parseTestNegative_unreachable_2(d *Doc, args Args) bool {
	if d.FieldFloat == args.ArgFloat {
		return true
	}

	if d.FieldInt != args.ArgInt {
		if d.FieldString == "abc" {
			return true
		}
		return false
	} else {
		if d.FieldString == "def" {
			return false
		}
		return true
	}

	return false
}

// Error: filter always evaluates to false
func parseTestNegative_empty_rows(_ *Doc, _ Args) bool {
	return false
}

// Error: Filter function expects exactly two parameters. First is pointer to document type. Second is query arguments
func parseTestNegative_not_enough_params(_ *Doc) bool {
	return false
}

// Error: Document parameter should be of struct type, got: :float64
func parseTestNegative_require_struct(_ float64, _ Args) bool {
	return false
}

// Error: filter should have bool return type
func parseTestNegative_require_bool_return(_ *Doc, _ Args) {
}

// Error: Filter function expects exactly two parameters. First is pointer to document type. Second is query arguments
func parseTestNegative_multi_name(d, d1 *Doc) bool {
	return false
}

// Error: unsupported block statement:
//
//	switch{
//		case d.FieldInt == 1:
//			return true
//		case d.FieldFloat == args.ArgFloat:
//			return true
//	}
func parseTestNegative_switch(d *Doc, args Args) bool {
	switch {
	case d.FieldInt == 1:
		return true
	case d.FieldFloat == args.ArgFloat:
		return true
	}
	return false
}

func TestFiltersNegative(t *testing.T) {
	execTests(t, "parseTestNegative_", false)
}

func TestMain(m *testing.M) {
	util.Configure(util.LogConfig{Level: "debug", Format: "console"})

	Program = loadProgram([]string{"."})

	os.Exit(m.Run())
}
