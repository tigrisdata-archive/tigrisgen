package main

import "testing"

// Update:
//
//	{
//		"$decrement":{
//			"field_float":12.5
//		},
//		"$divide":{
//			"field_float":12.5
//		},
//		"$increment":{
//			"field_float":12.5,
//			"field_string":"abc",
//			"nested.field_int":18
//		},
//		"$multiply":{
//			"nested.field_arr.5.field_int":10,
//			"nested.field_arr.7.field_int":{{.ArgInt}}
//		},
//		"$set":{
//			"field_int":10,
//			"field_bool":true,
//			"field_arr.3.field_uuid":{{.ArgUUID}}
//		}
//	}
func parseUpdateFunc_1(d *Doc, args *Args) {
	d.FieldInt = 10
	d.FieldFloat += 12.5
	d.FieldFloat /= 12.5
	d.FieldFloat -= 12.5
	d.FieldBool = true
	d.FieldString += "abc"
	//	d.FieldArrFloat = append(d.FieldArrFloat, 15.6)
	d.Nested.FieldInt += 18
	d.Nested.FieldArr[5].FieldInt *= 10
	d.Nested.FieldArr[7].FieldInt *= args.ArgInt
	d.FieldArr[3].FieldUUID = args.ArgUUID
}

// Update:
//
//	{
//		"$increment":{
//			"field_float":12.5,
//			"field_string":"abc",
//			"nested.field_int":18
//		},
//		"$multiply":{
//			"nested.field_arr.5.field_int":10,
//			"nested.field_arr.7.field_int":{{.ArgInt}}
//		},
//		"$set":{
//			"field_int":10,
//			"field_bool":true,
//			"field_arr.3.field_uuid":{{.ArgUUID}}
//		}
//	}
func parseUpdateFunc_client_side(d *Doc, args *Args) {
	d.FieldInt = 10
	d.FieldFloat += 12.5
	d.FieldBool = true
	d.FieldString += "abc"
	//	d.FieldArrFloat = append(d.FieldArrFloat, 15.6)
	d.Nested.FieldInt += 18
	d.Nested.FieldArr[5].FieldInt *= 10
	d.Nested.FieldArr[7].FieldInt *= args.ArgInt
	d.FieldArr[3].FieldUUID = args.ArgUUID
}

// Update: {"$set":{"field_float":{{.}}}}
func parseUpdateFunc_simple_arg(d *Doc, arg float64) {
	d.FieldFloat = arg
}

func TestUpdateFunc(t *testing.T) {
	execTests(t, "parseUpdateFunc_", true)
}

// Error: Update should not return results
func parseUpdateFuncNegative_require_no_return_params(d *Doc, args *Args) bool {
	return false
}

func TestUpdateFuncNegative(t *testing.T) {
	execTests(t, "parseUpdateFuncNegative_", true)
}
