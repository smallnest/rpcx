// Package parser parses Go code and keeps track of all the types defined
// and provides access to all the constants defined for an int type.
package parser

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"unicode"
	"unicode/utf8"
)

type Parser struct {
	PkgPath     string
	PkgName     string
	PkgFullName string
	StructNames []string
}

type visitor struct {
	*Parser

	name     string
	explicit bool
}

func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

func (v *visitor) Visit(n ast.Node) (w ast.Visitor) {
	switch n := n.(type) {
	case *ast.Package:
		return v
	case *ast.File:
		v.PkgName = n.Name.String()
		return v

	case *ast.GenDecl:
		return v
	case *ast.TypeSpec:
		v.name = n.Name.String()

		// Allow to specify non-structs explicitly independent of '-all' flag.
		if v.explicit {
			v.StructNames = append(v.StructNames, v.name)
			return nil
		}
		return v
	case *ast.StructType:
		if isExported(v.name) {
			v.StructNames = append(v.StructNames, v.name)
		}
		return nil
	}
	return nil
}

func (p *Parser) Parse(fname string, isDir bool) error {
	p.PkgPath = build.Default.GOPATH

	fset := token.NewFileSet()
	if isDir {
		packages, err := parser.ParseDir(fset, fname, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		for _, pckg := range packages {
			ast.Walk(&visitor{Parser: p}, pckg)
		}
	} else {
		f, err := parser.ParseFile(fset, fname, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		ast.Walk(&visitor{Parser: p}, f)
	}
	return nil
}
