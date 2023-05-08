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
	"go/ast"
	"go/types"

	"github.com/rs/zerolog/log"
	"github.com/tigrisdata/tigrisgen/util"
	"golang.org/x/tools/go/packages"
)

// types.Identical takes into account Pos, which is different for
// packages loaded separately, so doesn't detect identical types.
func identical(lName string, l *types.Signature, rName string, r *types.Signature) bool {
	if lName != rName {
		return false
	}

	if l.Params().Len() != r.Params().Len() {
		return false
	}

	if l.Results().Len() != r.Results().Len() {
		return false
	}

	if l.Recv() == nil && r.Recv() != nil || l.Recv() != nil && r.Recv() == nil {
		return false
	}

	for i := 0; i < l.Params().Len(); i++ {
		if l.Params().At(i).Type().String() != r.Params().At(i).Type().String() {
			return false
		}

		if l.Params().At(i).Name() != r.Params().At(i).Name() {
			return false
		}
	}

	for i := 0; i < l.Results().Len(); i++ {
		if l.Results().At(i).Type().String() != r.Results().At(i).Type().String() {
			return false
		}
	}

	return true
}

func signatureToFuncDecl(name string, sig *types.Signature, pi *packages.Package) (string, *ast.FuncDecl) {
	for _, f := range pi.Syntax {
		for _, v := range f.Decls {
			if fn, ok := v.(*ast.FuncDecl); ok {
				obj := pi.TypesInfo.ObjectOf(fn.Name)

				fnSig, ok := obj.Type().(*types.Signature)
				if !ok {
					FatalWithExpr(pi, v, "expected function signature")
				}

				if obj != nil && identical(name, sig, fn.Name.Name, fnSig) &&
					(sig.Recv() != nil && fnSig.Recv() != nil || sig.Recv() == nil && fnSig.Recv() == nil) {
					name = fn.Name.Name

					if fnSig.Recv() != nil {
						tn, ok := fnSig.Recv().Type().(*types.Named)
						if !ok {
							FatalWithExpr(pi, v, "expected named type")
						}

						if fnSig.Recv().Pkg().Path() == "." {
							name = fnSig.Recv().Pkg().Name() + "." + tn.Obj().Name() + "." + name
						} else {
							name = fnSig.Recv().Pkg().Path() + "." + tn.Obj().Name() + "." + name
						}
					} else if pi.PkgPath == "." {
						name = pi.Name + "." + name
					} else {
						name = pi.PkgPath + "." + name
					}

					return name, fn
				}
			}
		}
	}

	util.Fatal("function declaration %v='%v' not found in package '%v'", name, sig.String(), pi.PkgPath)

	return "", nil
}

func exprToFuncDecl(tp string, f ast.Expr, pi *packages.Package) (string, *ast.FuncDecl, *packages.Package) {
	switch a := f.(type) {
	case *ast.Ident: // function
		if a.Obj != nil && a.Obj.Kind == ast.Fun {
			log.Debug().Str("API", tp).Str("name", a.Name).Int("pos", int(a.Pos())).Msg("detected simple function")
			return pi.Name + "." + a.Name, a.Obj.Decl.(*ast.FuncDecl), pi
		}
	case *ast.SelectorExpr:
		if s, ok := a.X.(*ast.Ident); ok { // method or external package function
			if pkg, ok := pi.TypesInfo.ObjectOf(s).(*types.PkgName); ok {
				sig, ok := pi.TypesInfo.TypeOf(f).(*types.Signature)
				if !ok {
					util.Fatal("not a function parameter: %v", pi.TypesInfo.TypeOf(f).String())
				}

				path := pkg.Imported().Path()
				log.Debug().Str("API", tp).Str("type", pi.TypesInfo.TypeOf(f).String()).Str("package", path).
					Msg("detected external package function")

				if Program[path] == nil {
					loadProgram(Program, []string{path})
				}

				nm, decl := signatureToFuncDecl(a.Sel.Name, sig, Program[path])

				return nm, decl, Program[path]
			}

			fn, ok := pi.TypesInfo.ObjectOf(a.Sel).(*types.Func)
			if !ok {
				util.Fatal("not a function parameter: %v", pi.TypesInfo.Types[f].Type.String())
			}

			log.Debug().Str("API", tp).Str("name", a.Sel.Name).Int("pos", int(a.Pos())).Msg("detected document method")

			nm, decl := signatureToFuncDecl(a.Sel.Name, fn.Type().(*types.Signature), pi)

			return nm, decl, pi
		} else if sse, ok := a.X.(*ast.SelectorExpr); ok { // external package method
			if pkg, ok := pi.TypesInfo.ObjectOf(sse.X.(*ast.Ident)).(*types.PkgName); ok {
				fn, ok := pi.TypesInfo.ObjectOf(a.Sel).(*types.Func)
				if !ok {
					util.Fatal("not a function parameter: %v", pi.TypesInfo.ObjectOf(a.Sel).(*types.Func))
				}
				sig, ok := fn.Type().(*types.Signature)
				if !ok {
					util.Fatal("not a signature type: %v", fn.Type())
				}
				path := pkg.Imported().Path()
				log.Debug().Str("API", tp).Str("name", sig.String()).Str("package", path).
					Msg("detected external package document method")

				if Program[path] == nil {
					loadProgram(Program, []string{path})
				}

				nm, decl := signatureToFuncDecl(a.Sel.Name, sig, Program[path])

				return nm, decl, Program[path]
			}

			util.Fatal("unsupported API function parameter '%v.%v.%v'", sse.X, sse.Sel, a.Sel)
		}
	case *ast.FuncLit:
		//		FatalWithExpr(a, "Closures are not supported yet")
		return "anonymous", &ast.FuncDecl{Type: a.Type, Body: a.Body}, pi
	}

	FatalWithExpr(pi, f, "unsupported API function parameter")

	return "", nil, nil
}
