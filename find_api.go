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
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"reflect"
	"runtime"

	"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog/log"
	"github.com/tigrisdata/tigris-client-go/tigrisgen/util"
	"golang.org/x/tools/go/loader"
)

func signatureToFuncDecl(name string, sig *types.Signature, pi *loader.PackageInfo) (string, *ast.FuncDecl) {
	for _, f := range pi.Files {
		for _, v := range f.Decls {
			if fn, ok := v.(*ast.FuncDecl); ok {
				/*
					spew.Dump(fn.X.X)
					if fn.Recv != nil {
						spew.Dump(fn.Recv.List[0].Names[0])
					}
				*/
				obj := pi.Info.ObjectOf(fn.Name)
				fnSig := obj.Type().(*types.Signature)
				/*
					if fn.X.X == "FilterOne" || fn.X.X == "UpdateOne" {
						log.Debug().Msg("aaaaaaaaaaaaaaa")
						spew.Dump(pi.ObjectOf(fn.X))
						spew.Dump(sig)
						spew.Dump(sig.Recv())
						spew.Dump(fnSig.Recv())
						log.Debug().Msg("aaaaaaaaaaaaaaa")
					}
				*/
				if obj != nil && types.Identical(sig, fnSig) &&
					(sig.Recv() != nil && fnSig.Recv() != nil || sig.Recv() == nil && fnSig.Recv() == nil) {
					name = fn.Name.Name
					if fnSig.Recv() != nil {
						if fnSig.Recv().Pkg().Path() == "." {
							name = fnSig.Recv().Pkg().Name() + "." + fnSig.Recv().Type().(*types.Named).Obj().Name() + "." + name
						} else {
							name = fnSig.Recv().Pkg().Path() + "." + fnSig.Recv().Type().(*types.Named).Obj().Name() + "." + name
						}
					} else if pi.Pkg.Path() == "." {
						name = pi.Pkg.Name() + "." + name
					} else {
						name = pi.Pkg.Path() + "." + name
					}
					return name, fn
				}
			}
		}
	}

	util.Fatal("function declaration %v='%v' not found in package '%v'", name, sig.String(), pi.Pkg.Path())

	return "", nil
}

func exprToFuncDecl(tp string, f ast.Expr, pi *loader.PackageInfo, p *loader.Program) (string, *ast.FuncDecl) {
	switch a := f.(type) {
	case *ast.Ident: // function
		if a.Obj != nil && a.Obj.Kind == ast.Fun {
			log.Debug().Str("API", tp).Str("name", a.Name).Int("pos", int(a.Pos())).Msg("detected simple function")
			spew.Dump(pi.Pkg.Name())
			return pi.Pkg.Name() + "." + a.Name, a.Obj.Decl.(*ast.FuncDecl)
		}
	case *ast.SelectorExpr:
		//		spew.Dump(pi.Selections[a])
		if s, ok := a.X.(*ast.Ident); ok { // method or external package function
			if pkg, ok := pi.ObjectOf(s).(*types.PkgName); ok {
				sig, ok := pi.Types[f].Type.(*types.Signature)
				if !ok {
					util.Fatal("not a function parameter: %v", pi.Types[f].Type.String())
				}
				path := pkg.Imported().Path()
				/*
					log.Debug().Msg("iiiiiiiiiiiiiiiiiiiiiii")
					spew.Dump(sig)
					spew.Dump(p.Package(path).Types[f])
					log.Debug().Msg("iiiiiiiiiiiiiiiiiiiiiii")
				*/
				log.Debug().Str("API", tp).Str("type", pi.Types[f].Type.String()).Str("package", path).
					Msg("detected external package function")
				return signatureToFuncDecl(a.Sel.Name, sig, p.Package(path))
			}

			fn, ok := pi.ObjectOf(a.Sel).(*types.Func)
			if !ok {
				util.Fatal("not a function parameter: %v", pi.Types[f].Type.String())
			}
			/*
				log.Debug().Msg("dddddddddddddddddddd")
				spew.Dump(pi.ObjectOf(a.Sel).(*types.Func).Type().(*types.Signature))
				spew.Dump(pi.Info.Types[f].Type)
				spew.Dump(pi.Types[f].Type.(*types.Signature))
				log.Debug().Msg("dddddddddddddddddddd")
			*/
			//return signatureToFuncDecl(a.Sel.X, pi.Types[f].Type.(*types.Signature), pi)
			log.Debug().Str("API", tp).Str("name", a.Sel.Name).Int("pos", int(a.Pos())).Msg("detected document method")
			return signatureToFuncDecl(a.Sel.Name, fn.Type().(*types.Signature), pi)
		} else if sse, ok := a.X.(*ast.SelectorExpr); ok { // external package method
			if pkg, ok := pi.ObjectOf(sse.X.(*ast.Ident)).(*types.PkgName); ok {
				/*
					log.Debug().Msg("dddddddddddddddddddd")
					spew.Dump(pi.ObjectOf(a.Sel).(*types.Func).Type().(*types.Signature))
					spew.Dump(pi.Types[f].Type.(*types.Signature))
					log.Debug().Msg("dddddddddddddddddddd")
				*/
				fn, ok := pi.ObjectOf(a.Sel).(*types.Func)
				if !ok {
					util.Fatal("not a function parameter: %v", pi.ObjectOf(a.Sel).(*types.Func))
				}
				sig, ok := fn.Type().(*types.Signature)
				if !ok {
					util.Fatal("not a signature type: %v", fn.Type())
				}
				path := pkg.Imported().Path()
				/*
					log.Debug().Msg("iiiiiiiiiiiiiiiiiiiiiii")
					spew.Dump(sig)
					spew.Dump(a)
					spew.Dump(pi.ObjectOf(a.Sel).(*types.Func).Type().(*types.Signature).Recv())
					spew.Dump(p.Package(path).Types[a])
					spew.Dump(sig.Recv())
					log.Debug().Msg("iiiiiiiiiiiiiiiiiiiiiii")

				*/
				log.Debug().Str("API", tp).Str("name", sig.String()).Str("package", path).
					Msg("detected external package document method")
				return signatureToFuncDecl(a.Sel.Name, sig, p.Package(pkg.Imported().Path()))
			}

			util.Fatal("unsupported API function parameter '%v.%v.%v'", sse.X, sse.Sel, a.Sel)
		}
	case *ast.FuncLit:
		fmt.Fprintf(os.Stderr, "aaaaaaaaaaaaaaaaaaaaaaa\n")

		name := runtime.FuncForPC(reflect.ValueOf(a.Type).Pointer()).Name()

		fmt.Fprintf(os.Stderr, "aaaaaaaaaaaaaaaaaaaaaaa %v\n", name)
		return "anonymous", &ast.FuncDecl{Type: a.Type, Body: a.Body}
	default:
		//		spew.Dump(a)
	}

	util.Fatal("unsupported API function parameter type='%v'", reflect.TypeOf(f))

	return "", nil
}
