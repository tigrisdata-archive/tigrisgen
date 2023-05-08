package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	flts := []FilterDef{
		{Name: "main.FilterOne", Body: `{"Field3":{"$lte":{{.}}}}`},
		{Name: "main.FilterTwo", Body: `{"Field2":{"$lt":10}}`},
	}

	upds := []FilterDef{
		{Name: "main.UpdateOne", Body: `{"$decrement":{"field_float":12.5}}`},
		{Name: "main.UpdateTwo", Body: `{"$multiply":{"nested.field_arr.5.field_int":10, "nested.field_arr.7.field_int":{{.ArgInt}}}`},
	}

	var buf bytes.Buffer

	err := writeGenFileLow(&buf, "pkg_todo", flts, upds)
	require.NoError(t, err)

	require.Equal(t, "", buf.String())
}
