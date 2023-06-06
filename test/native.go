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

import "context"

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

type NativeCollection[T any, P any] struct{}

type Response struct{}

// Below is for API lookup test.
func Update[T, P any, F any, U any](ctx context.Context, c *NativeCollection[T, P], filter func(T, F) bool,
	update func(T, U), args F, uargs U,
) (*Response, error) {
	return &Response{}, nil
}

// UpdateOne partially updates first document matching the filter.
func UpdateOneAPI[T, P any, F any, U any](ctx context.Context, c *NativeCollection[T, P], filter func(T, F) bool,
	update func(T, U), args F, uargs U,
) (*Response, error) {
	return &Response{}, nil
}

// Read returns documents which satisfies the filter.
func Read[T, P any, F any](ctx context.Context, c *NativeCollection[T, P], filter func(T, F) bool, args F,
) (*Response, error) {
	return &Response{}, nil
}

// ReadOne reads one document from the collection satisfying the filter.
func ReadOne[T, P any, F any](ctx context.Context, c *NativeCollection[T, P], filter func(T, F) bool, args F,
) (*P, error) {
	var p P
	return &p, nil
}

func ReadWithOptions[T, P any, F any](ctx context.Context, c *NativeCollection[T, P], filter func(T, F) bool, args F,
	options *Response,
) (*Response, error) {
	return &Response{}, nil
}

// Delete removes documents from the collection according to the filter.
func Delete[T, P any, F any](ctx context.Context, c *NativeCollection[T, P], filter func(T, F) bool, args F,
) (*Response, error) {
	return &Response{}, nil
}

// DeleteOne deletes first document satisfying the filter.
func DeleteOne[T, P any, F any](ctx context.Context, c *NativeCollection[T, P], filter func(T, F) bool, args F,
) (*Response, error) {
	return &Response{}, nil
}
