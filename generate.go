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
	_ "embed"
	"io"
	"os"
	"text/template"

	"github.com/rs/zerolog/log"
)

//go:embed tigris.gen.gotmpl
var genTempl string

type FilterDef struct {
	Name string
	Body string
}

type vars struct {
	Package string
	Filters []FilterDef
	Updates []FilterDef
}

func writeGenFileLow(w io.Writer, pkg string, filters []FilterDef, updates []FilterDef) error {
	t, err := template.New("exec_template").Parse(genTempl)
	if err != nil {
		return err
	}

	v := vars{
		Package: pkg,
		Filters: filters,
		Updates: updates,
	}

	return t.Execute(w, v)
}

func writeGenFile(name string, pkg string, filters []FilterDef, updates []FilterDef) error {
	log.Info().Str("file_name", name).Str("package", pkg).Msg("generating")

	f, err := os.Create(name)
	if err != nil {
		return err
	}

	if err = writeGenFileLow(f, pkg, filters, updates); err != nil {
		_ = f.Close()
		return err
	}

	return f.Close()
}
