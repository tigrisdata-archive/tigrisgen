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

package test

// For out package native API tests
// See tigris.TestNativeAPI

type Nested struct {
	Field222 string `json:"field_222"`
}

type Doc struct {
	Field1 int
	Field2 int
	Field3 float64

	Nested Nested `json:"nested"`
}

type Args struct {
	F1 int
	F2 int
}

func FilterOne(d Doc, _ float64) bool {
	return d.Field1 != 10 && d.Field2 > 99
}

/*
func FilterOnePtr(d *Doc, i float64) bool {
	return d.Field1 != 10 && d.Field2 > 100
}
*/

func (d Doc) FilterOne(args float64) bool {
	return d.Field1 != 10 && d.Field2 > 111
}

/*
func (d *Doc) FilterOnePtr(args float64) bool {
	return d.Field1 != 10 && d.Field2 > 100
}
*/

func UpdateOne(d Doc, i int) {
	d.Field1 += i
}

/*
func UpdateOnePtr(d *Doc, i int) {
	d.Field1 += i
}
*/

func (d Doc) UpdateOne(args int) {
	d.Field2 -= 10 //nolint:staticcheck
}

/*
func (d *Doc) UpdateOnePtr(args int) {
	d.Field1 += args
}
*/
