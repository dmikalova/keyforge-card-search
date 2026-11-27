//go:build mage

// gallery.keyforge.cards developer tasks. The ci namespace comes from project-standards'
// shared ci package. Run `mage -l` to list them, and
// `mage ci:fix && mage ci:check` before calling work done.
package main

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	// mage:import ci
	"github.com/dmikalova/project-standards/ci"
	"github.com/magefile/mage/sh"
)

// templDir holds the .templ files. Their generated _templ.go files are
// committed, so `go build` works without the templ CLI.
const templDir = "internal/web"

func init() {
	ci.ExtraBuilds = append(ci.ExtraBuilds, templCurrent)
}

// Templ regenerates the Go code for the .templ files, then runs ci:fix so the
// generated files are in the gate's format. Commit the result.
func Templ() error {
	if err := sh.RunV("go", "tool", "templ", "generate", "-path", templDir); err != nil {
		return err
	}

	return ci.Fix()
}

// templCurrent fails when a .templ file changed without `mage templ`. It
// generates each file to stdout and compares it with the committed file. The
// gate's formatters regroup the imports, so the comparison ignores the import
// layout and compares the import set and the rest of the code.
func templCurrent(context.Context) error {
	sources, err := filepath.Glob(filepath.Join(templDir, "*.templ"))
	if err != nil {
		return err
	}

	var stale []string

	for _, src := range sources {
		// Run from templDir: templ writes the path it was given into the
		// generated error messages, and `generate -path` uses the bare name.
		cmd := exec.Command("go", "tool", "templ", "generate", "-stdout", "-f", filepath.Base(src))
		cmd.Dir = templDir

		fresh, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("generate %s: %w", src, err)
		}

		target := strings.TrimSuffix(src, ".templ") + "_templ.go"

		committed, err := os.ReadFile(target)
		if err != nil {
			return fmt.Errorf("%s has no generated file (run mage templ): %w", src, err)
		}

		same, err := sameCode(fresh, committed)
		if err != nil {
			return fmt.Errorf("compare %s: %w", target, err)
		}

		if !same {
			stale = append(stale, target)
		}
	}

	if len(stale) > 0 {
		return fmt.Errorf(
			"generated templ code is stale (run mage templ): %s",
			strings.Join(stale, ", "),
		)
	}

	return nil
}

// sameCode reports whether two Go files have the same imports and the same
// code once both are formatted.
func sameCode(a, b []byte) (bool, error) {
	ia, ca, err := split(a)
	if err != nil {
		return false, err
	}

	ib, cb, err := split(b)
	if err != nil {
		return false, err
	}

	return slices.Equal(ia, ib) && slices.Equal(tokens(ca), tokens(cb)), nil
}

// tokens lists the code's tokens, leaving out what line wrapping changes:
// newlines and the trailing comma before a closing bracket.
func tokens(src []byte) []string {
	fset := token.NewFileSet()

	var s scanner.Scanner

	s.Init(fset.AddFile("", fset.Base(), len(src)), src, nil, 0)

	var out []string

	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			return out
		}

		if tok == token.SEMICOLON && lit == "\n" {
			continue
		}

		closing := tok == token.RPAREN || tok == token.RBRACE || tok == token.RBRACK
		if closing && len(out) > 0 && out[len(out)-1] == token.COMMA.String() {
			out = out[:len(out)-1]
		}

		out = append(out, tok.String()+lit)
	}
}

// split returns a file's sorted import specs and its formatted code without
// the import declarations or comments.
func split(src []byte) ([]string, []byte, error) {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return nil, nil, err
	}

	var imports []string

	for _, spec := range f.Imports {
		name := ""
		if spec.Name != nil {
			name = spec.Name.Name + " "
		}

		imports = append(imports, name+spec.Path.Value)
	}

	slices.Sort(imports)

	f.Decls = slices.DeleteFunc(f.Decls, func(d ast.Decl) bool {
		g, ok := d.(*ast.GenDecl)

		return ok && g.Tok == token.IMPORT
	})
	f.Imports = nil

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, nil, err
	}

	return imports, buf.Bytes(), nil
}
