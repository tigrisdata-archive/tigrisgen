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
	"fmt"
	"go/ast"
	"go/printer"
	"go/types"
	"os"
	"reflect"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/tigrisdata/tigrisgen/util"
	"golang.org/x/tools/go/packages"
)

var tigrisPkg = "github.com/tigrisdata/tigris-client-go/tigris"

// Program source loaded into memory.
var (
	Program = map[string]*packages.Package{}
	Pwd     string
)

func getFilterFunc(name string, pi *packages.Package, ident ast.Expr, flt ast.Expr, upd ast.Expr, filtersFNs []ast.Expr,
	updatesFNs []ast.Expr,
) ([]ast.Expr, []ast.Expr) {
	if ssse, ok := ident.(*ast.Ident); ok {
		if pkg, ok := pi.TypesInfo.ObjectOf(ssse).(*types.PkgName); ok {
			if pkg.Imported().Path() == tigrisPkg {
				filtersFNs = append(filtersFNs, flt)

				if upd != nil {
					updatesFNs = append(updatesFNs, upd)
				}

				log.Debug().Str("API", name).Str("filter", pi.TypesInfo.Types[flt].Type.String()).
					Str("update", pi.TypesInfo.Types[upd].Type.String()).Msg("get params")
			}
		}
	}

	return filtersFNs, updatesFNs
}

func findAPIcalls(node *ast.File, pi *packages.Package, apiName string) ([]ast.Expr, []ast.Expr) {
	filtersFNs := make([]ast.Expr, 0)
	updatesFNs := make([]ast.Expr, 0)

	ast.Inspect(node, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if ok {
			var arg2, arg3 ast.Expr
			if len(ce.Args) > 2 {
				arg2 = ce.Args[2]
			}

			if len(ce.Args) > 3 {
				arg3 = ce.Args[3]
			}

			// tigris.Update(...
			if se, ok := ce.Fun.(*ast.SelectorExpr); ok {
				if se.Sel.Name == apiName {
					filtersFNs, updatesFNs = getFilterFunc(apiName, pi, se.X, arg2, arg3, filtersFNs, updatesFNs)
				}
			}
			// tigris.Update[Doc](...
			if se, ok := ce.Fun.(*ast.IndexExpr); ok {
				if sse, ok := se.X.(*ast.SelectorExpr); ok {
					if sse.Sel.Name == apiName {
						filtersFNs, updatesFNs = getFilterFunc(apiName, pi, sse.X, arg2, arg3, filtersFNs, updatesFNs)
					}
				}
			}
			// tigris.Update[Doc, Args](...
			if se, ok := ce.Fun.(*ast.IndexListExpr); ok {
				if sse, ok := se.X.(*ast.SelectorExpr); ok {
					if sse.Sel.Name == apiName {
						filtersFNs, updatesFNs = getFilterFunc(apiName, pi, sse.X, arg2, arg3, filtersFNs, updatesFNs)
					}
				}
			}

			return true
		}

		return true
	})

	return filtersFNs, updatesFNs
}

func loadProgram(program map[string]*packages.Package, args []string) {
	start := time.Now()

	cfg := packages.Config{Tests: true, Mode: packages.NeedName | packages.NeedDeps | packages.NeedSyntax |
		packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule}

	pkgs, err := packages.Load(&cfg, args...)
	if err != nil {
		panic(err)
	}

	for _, v := range pkgs {
		log.Info().Str("id", v.ID).Str("path", v.PkgPath).Str("name", v.Name).Msg("loading package")
		program[v.ID] = v
	}

	if packages.PrintErrors(pkgs) != 0 {
		os.Exit(1)
	}

	log.Info().Dur("duration", time.Since(start)).Msg("parse time")
}

func findAndParseAPI(api string, f *ast.File, pi *packages.Package,
	filters []FilterDef, updates []FilterDef,
	fltName map[string]bool, updName map[string]bool,
) ([]FilterDef, []FilterDef) {
	log.Debug().Str("API", api).Msg("parsing")

	flt, u := findAPIcalls(f, pi, api)

	if api != "UpdateAll" {
		for _, ff := range flt {
			name, body, pi := exprToFuncDecl(api, ff, pi)
			if body != nil {
				if fltName[name] {
					log.Debug().Str("name", name).Msg("skipping duplicate filter")
					continue
				}

				fltName[name] = true
				n, flt := parseFilterFunction(name, body, pi)
				log.Info().Str("name", n).Str("filter", flt).Msg("filter")
				filters = append(filters, FilterDef{n, flt})
			} else {
				log.Warn().Str("package", pi.Name).Str("expr",
					reflect.TypeOf(ff).Name()).Msg("not a filter function")
			}
		}
	}

	if api != "Update" && api != "UpdateOne" && api != "UpdateAll" {
		return filters, updates
	}

	if api == "UpdateAll" {
		u = flt
	}

	for _, ff := range u {
		name, body, pi := exprToFuncDecl(api, ff, pi)
		if body != nil {
			if updName[name] {
				log.Debug().Str("name", name).Msg("skipping duplicate update")
				continue
			}

			updName[name] = true
			n, upd := parseUpdateFunction(name, body, pi)
			log.Info().Str("name", n).Str("update", upd).Msg("update")
			updates = append(updates, FilterDef{n, upd})
		} else {
			log.Warn().Str("package", pi.Name).Str("expr",
				reflect.TypeOf(ff).Name()).Msg("not an update function")
		}
	}

	return filters, updates
}

func findAndParse(pi *packages.Package) ([]FilterDef, []FilterDef) {
	var (
		filters []FilterDef
		updates []FilterDef
	)

	// deduplicate functions
	fltName := make(map[string]bool)
	updName := make(map[string]bool)

	log.Debug().Str("package", pi.Name).Msg("processing package")

	apis := []string{"Update", "UpdateOne", "UpdateAll", "Read", "ReadOne", "ReadWithOptions", "Delete", "DeleteOne"}

	for _, f := range pi.Syntax {
		log.Debug().Str("file", pi.Fset.File(f.Pos()).Name()).Msg("processing file")

		for _, v := range apis {
			filters, updates = findAndParseAPI(v, f, pi, filters, updates, fltName, updName)
		}
	}

	return filters, updates
}

func MainLow() {
	util.Configure(util.LogConfig{Format: "console", Level: "info"})

	s, _ := os.Getwd()
	log.Debug().Strs("args", os.Args).Str("pwd", s).Msg("Starting")

	var err error

	Pwd, err = os.Getwd()
	if err != nil {
		util.Fatal("%v", err)
	}

	loadProgram(Program, []string{Pwd})

	for _, pi := range Program {
		if pi.Name != os.Getenv("GOPACKAGE") {
			continue
		}

		log.Debug().Str("package", pi.Name).Msg("processing")

		flts, upds := findAndParse(pi)

		if len(flts) == 0 && len(upds) == 0 {
			log.Debug().Msg("No filters or updates found in package")
			continue
		}

		log.Debug().Interface("filters", flts).Interface("updates", upds).Msg("parsed")

		err := writeGenFile("tigris.gen.go", pi.Name, flts, upds)
		if err != nil {
			panic(err)
		}
	}

	log.Debug().Msg("Finished")
}

func fatalWithExpr(pi *packages.Package, e ast.Node, format string, args ...any) {
	if Program != nil {
		pos := pi.Fset.Position(e.Pos())
		log.Error().CallerSkipFrame(1).Str("line", pos.String()).Msgf(format, args...)

		cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}

		fmt.Fprintf(os.Stderr, ">>>>>>>>>>>>>>>>>>>>>>\n")
		_ = cfg.Fprint(os.Stderr, pi.Fset, e)
		fmt.Fprintf(os.Stderr, "\n>>>>>>>>>>>>>>>>>>>>>>\n")
	} else {
		log.Error().CallerSkipFrame(1).Msgf(format, args...)
	}

	os.Exit(1)
}

var FatalWithExpr = fatalWithExpr
