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

import (
	"testing"
	"time"
)

// Update:
//
//	{
//		"$set":{
//			"field_int":10,
//			"field_bool":true,
//			"field_arr.3.field_uuid":{{toJSON .Arg.ArgUUID}},
//			"FieldMap.abc":10.5,
//			"FieldMapInt.77":"val1",
//			"nested.FieldMap.def":11.5,
//			"nested.FieldMapInt.88":{{toJSON .Arg.ArgString}},
//			"field_time":{{toJSON .Time}}
//		},
//		"$increment":{
//			"field_float":12.5,
//			"field_string":"abc",
//			"nested.field_int":18
//		},
//		"$decrement":{
//			"field_float":12.5
//		},
//		"$divide":{
//			"field_float":12.5
//		},
//		"$multiply":{
//			"nested.field_arr.5.field_int":10,
//			"nested.field_arr.7.field_int":{{toJSON .Arg.ArgInt}}
//		},
//		"$push":{
//			"field_arr_float":8.8,
//			"field_arr_float":{{toJSON .Arg.ArgFloat}}
//		}
//	}
func parseUpdateFunc_1(d *Doc, args *Args) {
	d.FieldInt = 10
	d.FieldFloat += 12.5
	d.FieldFloat /= 12.5
	d.FieldFloat -= 12.5
	d.FieldBool = true
	d.FieldString += "abc"
	d.Nested.FieldInt += 18
	d.Nested.FieldArr[5].FieldInt *= 10
	d.Nested.FieldArr[7].FieldInt *= args.ArgInt
	d.FieldArr[3].FieldUUID = args.ArgUUID
	d.FieldMap["abc"] = 10.5
	d.FieldMapInt[77] = "val1"
	d.Nested.FieldMap["def"] = 11.5
	d.Nested.FieldMapInt[88] = args.ArgString

	d.FieldArrFloat = append(d.FieldArrFloat, 8.8)
	d.FieldArrFloat = append(d.FieldArrFloat, args.ArgFloat)

	d.FieldTime = time.Now()
}

// Update:
//
//	{
//		"$set":{
//			"field_int":10,
//			"field_bool":true,
//			"field_arr.3.field_uuid":{{toJSON .Arg.ArgUUID}}
//		},
//		"$increment":{
//			"field_float":12.5,
//			"field_string":"abc",
//			"nested.field_int":18
//		},
//		"$multiply":{
//			"nested.field_arr.5.field_int":10,
//			"nested.field_arr.7.field_int":{{toJSON .Arg.ArgInt}}
//		}
//	}
func parseUpdateFunc_client_side(d *Doc, args *Args) {
	d.FieldInt = 10
	d.FieldFloat += 12.5
	d.FieldBool = true
	d.FieldString += "abc"
	//	d.FieldArrFloat = append(d.FieldArrFloat, 15.6)
	d.Nested.FieldInt += 18
	//	if args.ArgFloat == 1.4 {
	d.Nested.FieldArr[5].FieldInt *= 10
	//	}
	//	if testConstString == args.ArgString {
	d.Nested.FieldArr[7].FieldInt *= args.ArgInt
	//	}
	d.FieldArr[3].FieldUUID = args.ArgUUID
}

// Update:
//
//	{"$set":{"field_float":{{toJSON .Arg}}}}
func parseUpdateFunc_simple_arg(d *Doc, arg float64) {
	d.FieldFloat = arg
}

func TestUpdateFunc(t *testing.T) {
	execTests(t, "parseUpdateFunc_", true)
}

var (
	_ = parseUpdateFuncNegative_require_no_return_params
	_ = parseUpdateFuncNegative_lhs_and_append_left_arg_must_be_same
)

// Error:
//
//	Update should not return results
func parseUpdateFuncNegative_require_no_return_params(d *Doc, args *Args) bool {
	return false
}

// Error:
//
//	Unsupported update statement: d.FieldArrFloat = append(d.FieldArr[0].FieldArrFloat, 8.8)
func parseUpdateFuncNegative_lhs_and_append_left_arg_must_be_same(d *Doc, _ *Args) {
	d.FieldArrFloat = append(d.FieldArr[0].FieldArrFloat, 8.8)
}

func TestUpdateFuncNegative(t *testing.T) {
	execTests(t, "parseUpdateFuncNegative_", true)
}
