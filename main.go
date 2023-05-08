package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/types"
	"os"
	"reflect"
	"time"

	"github.com/tigrisdata/tigris-client-go/tigrisgen/util"

	"github.com/rs/zerolog/log"

	"golang.org/x/tools/go/loader"
)

const tigrisPkg = "github.com/tigrisdata/tigris-client-go/tigris"

// Program source loaded into memory
var Program *loader.Program

func getFilterFunc(name string, pi *loader.PackageInfo, ident ast.Expr, flt ast.Expr, upd ast.Expr, filtersFNs []ast.Expr,
	updatesFNs []ast.Expr,
) ([]ast.Expr, []ast.Expr) {
	if ssse, ok := ident.(*ast.Ident); ok {
		if pkg, ok := pi.ObjectOf(ssse).(*types.PkgName); ok {
			if pkg.Imported().Path() == tigrisPkg {
				filtersFNs = append(filtersFNs, flt)
				if upd != nil {
					updatesFNs = append(updatesFNs, upd)
				}
				log.Debug().Str("API", name).Str("filter", pi.Types[flt].Type.String()).
					Str("update", pi.Types[upd].Type.String()).Msg("get params")
			}
		}
	}

	return filtersFNs, updatesFNs
}

func findMethods(node *ast.File, pi *loader.PackageInfo) map[string]*ast.FuncDecl {
	res := make(map[string]*ast.FuncDecl)

	ast.Inspect(node, func(n ast.Node) bool {
		if fun, ok := n.(*ast.FuncDecl); ok {
			if fun.Recv != nil && len(fun.Recv.List) == 1 {
				if r, rok := fun.Recv.List[0].Type.(*ast.StarExpr); rok {
					res[r.X.(*ast.Ident).Name+"."+fun.Name.Name] = fun
				} else if r, rok := fun.Recv.List[0].Type.(*ast.Ident); rok {
					res[r.Name+"."+fun.Name.Name] = fun
				}
			}
		}
		return true
	})

	return res
}

func findAPIcalls(node *ast.File, pi *loader.PackageInfo, apiName string) ([]ast.Expr, []ast.Expr) {
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

func loadProgram(args []string) *loader.Program {
	var conf loader.Config

	_, err := conf.FromArgs(args, true)
	if err != nil {
		panic(err)
	}

	conf.ParserMode = conf.ParserMode | parser.ParseComments

	start := time.Now()
	p, err := conf.Load()
	if err != nil {
		panic(err)
	}

	log.Info().Dur("duration", time.Since(start)).Msg("parse time")

	return p
}

func findAndParse(p *loader.Program, v *loader.PackageInfo) ([]FilterDef, []FilterDef) {
	var filters []FilterDef
	var updates []FilterDef

	log.Debug().Str("package", v.Pkg.Name()).Msg("processing package")

	for _, f := range v.Files {
		log.Debug().Str("file", f.Name.Name).Msg("processing package")
		f, u := findAPIcalls(f, v, "Update")

		for _, ff := range f {
			name, fnd := exprToFuncDecl("Update", ff, v, p)
			if fnd != nil {
				n, flt := parseFilterFunction(name, fnd, v)
				log.Debug().Str("name", n).Str("filter", flt).Msg("filter")
				filters = append(filters, FilterDef{n, flt})
			} else {
				log.Warn().Str("package", v.Pkg.Name()).Str("expr", reflect.TypeOf(ff).Name()).Msg("not a filter function")
			}
		}

		for _, ff := range u {
			name, body := exprToFuncDecl("Update", ff, v, p)
			if body != nil {
				n, upd := parseUpdateFunction(name, body, v)
				log.Debug().Str("name", n).Str("update", upd).Msg("update")
				updates = append(updates, FilterDef{n, upd})
			} else {
				log.Warn().Str("package", v.Pkg.Name()).Str("expr", reflect.TypeOf(ff).Name()).Msg("not an update function")
			}
		}
	}

	return filters, updates
}

func main() {
	util.Configure(util.LogConfig{Level: "debug"})

	s, _ := os.Getwd()
	log.Debug().Strs("args", os.Args).Str("pwd", s).Msg("Starting")

	args := []string{os.Getenv("GOFILE")}
	if len(os.Args) >= 2 {
		args = os.Args
	}

	p := loadProgram(args)

	for _, pi := range p.InitialPackages() {
		log.Debug().Str("package", pi.Pkg.Name()).Msg("processing")

		flts, upds := findAndParse(p, pi)

		log.Debug().Interface("filters", flts).Interface("updates", upds).Msg("parsed")

		err := writeGenFile("tigris.gen.go", pi.Pkg.Name(), flts, upds)
		if err != nil {
			panic(err)
		}
	}

	log.Debug().Msg("Finished")
}

func fatalWithExpr(e ast.Node, format string, args ...any) {
	if Program != nil {
		pos := Program.Fset.Position(e.Pos())
		log.Error().CallerSkipFrame(1).Str("line", pos.String()).Msgf(format, args...)
		cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}
		fmt.Fprintf(os.Stderr, ">>>>>>>>>>>>>>>>>>>>>>\n")
		_ = cfg.Fprint(os.Stderr, Program.Fset, e)
		fmt.Fprintf(os.Stderr, ">>>>>>>>>>>>>>>>>>>>>>\n")
	} else {
		log.Error().CallerSkipFrame(1).Msgf(format, args...)
	}

	os.Exit(1)
}

var FatalWithExpr = fatalWithExpr
