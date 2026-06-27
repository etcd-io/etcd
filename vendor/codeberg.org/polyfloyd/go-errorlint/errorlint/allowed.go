package errorlint

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"
)

type AllowPair struct {
	Err string
	Fun string
}

var allowedErrorsMap = make(map[string]map[string]struct{})

func setDefaultAllowedErrors() {
	allowedMapAppend([]AllowPair{
		// pkg/archive/tar
		{Err: "io.EOF", Fun: "(*archive/tar.Reader).Next"},
		{Err: "io.EOF", Fun: "(*archive/tar.Reader).Read"},
		// pkg/bufio
		{Err: "io.EOF", Fun: "(*bufio.Reader).Discard"},
		{Err: "io.EOF", Fun: "(*bufio.Reader).Peek"},
		{Err: "io.EOF", Fun: "(*bufio.Reader).Read"},
		{Err: "io.EOF", Fun: "(*bufio.Reader).ReadByte"},
		{Err: "io.EOF", Fun: "(*bufio.Reader).ReadBytes"},
		{Err: "io.EOF", Fun: "(*bufio.Reader).ReadLine"},
		{Err: "io.EOF", Fun: "(*bufio.Reader).ReadSlice"},
		{Err: "io.EOF", Fun: "(*bufio.Reader).ReadString"},
		{Err: "io.EOF", Fun: "(*bufio.Scanner).Scan"},
		// pkg/bytes
		{Err: "io.EOF", Fun: "(*bytes.Buffer).Read"},
		{Err: "io.EOF", Fun: "(*bytes.Buffer).ReadByte"},
		{Err: "io.EOF", Fun: "(*bytes.Buffer).ReadBytes"},
		{Err: "io.EOF", Fun: "(*bytes.Buffer).ReadRune"},
		{Err: "io.EOF", Fun: "(*bytes.Buffer).ReadString"},
		{Err: "io.EOF", Fun: "(*bytes.Reader).Read"},
		{Err: "io.EOF", Fun: "(*bytes.Reader).ReadAt"},
		{Err: "io.EOF", Fun: "(*bytes.Reader).ReadByte"},
		{Err: "io.EOF", Fun: "(*bytes.Reader).ReadRune"},
		{Err: "io.EOF", Fun: "(*bytes.Reader).ReadString"},
		// pkg/database/sql
		{Err: "database/sql.ErrNoRows", Fun: "(*database/sql.Row).Scan"},
		// pkg/debug/elf
		{Err: "io.EOF", Fun: "debug/elf.Open"},
		{Err: "io.EOF", Fun: "debug/elf.NewFile"},
		// pkg/io
		{Err: "io.EOF", Fun: "(io.ReadCloser).Read"},
		{Err: "io.EOF", Fun: "(io.Reader).Read"},
		{Err: "io.EOF", Fun: "(io.ReaderAt).ReadAt"},
		{Err: "io.EOF", Fun: "(*io.LimitedReader).Read"},
		{Err: "io.EOF", Fun: "(*io.SectionReader).Read"},
		{Err: "io.EOF", Fun: "(*io.SectionReader).ReadAt"},
		{Err: "io.ErrClosedPipe", Fun: "(*io.PipeWriter).Write"},
		{Err: "io.EOF", Fun: "io.ReadAtLeast"},
		{Err: "io.ErrShortBuffer", Fun: "io.ReadAtLeast"},
		{Err: "io.ErrUnexpectedEOF", Fun: "io.ReadAtLeast"},
		{Err: "io.EOF", Fun: "io.ReadFull"},
		{Err: "io.ErrUnexpectedEOF", Fun: "io.ReadFull"},
		// pkg/mime
		{Err: "mime.ErrInvalidMediaParameter", Fun: "mime.ParseMediaType"},
		// pkg/net/http
		{Err: "net/http.ErrServerClosed", Fun: "(*net/http.Server).ListenAndServe"},
		{Err: "net/http.ErrServerClosed", Fun: "(*net/http.Server).ListenAndServeTLS"},
		{Err: "net/http.ErrServerClosed", Fun: "(*net/http.Server).Serve"},
		{Err: "net/http.ErrServerClosed", Fun: "(*net/http.Server).ServeTLS"},
		{Err: "net/http.ErrServerClosed", Fun: "net/http.ListenAndServe"},
		{Err: "net/http.ErrServerClosed", Fun: "net/http.ListenAndServeTLS"},
		{Err: "net/http.ErrServerClosed", Fun: "net/http.Serve"},
		{Err: "net/http.ErrServerClosed", Fun: "net/http.ServeTLS"},
		// pkg/os
		{Err: "io.EOF", Fun: "(*os.File).Read"},
		{Err: "io.EOF", Fun: "(*os.File).ReadAt"},
		{Err: "io.EOF", Fun: "(*os.File).ReadDir"},
		{Err: "io.EOF", Fun: "(*os.File).Readdir"},
		{Err: "io.EOF", Fun: "(*os.File).Readdirnames"},
		// pkg/strings
		{Err: "io.EOF", Fun: "(*strings.Reader).Read"},
		{Err: "io.EOF", Fun: "(*strings.Reader).ReadAt"},
		{Err: "io.EOF", Fun: "(*strings.Reader).ReadByte"},
		{Err: "io.EOF", Fun: "(*strings.Reader).ReadRune"},
		// pkg/context
		{Err: "context.DeadlineExceeded", Fun: "(context.Context).Err"},
		{Err: "context.Canceled", Fun: "(context.Context).Err"},
		// pkg/encoding/json
		{Err: "io.EOF", Fun: "(*encoding/json.Decoder).Decode"},
		{Err: "io.EOF", Fun: "(*encoding/json.Decoder).Token"},
		// pkg/encoding/csv
		{Err: "io.EOF", Fun: "(*encoding/csv.Reader).Read"},
		// pkg/mime/multipart
		{Err: "io.EOF", Fun: "(*mime/multipart.Reader).NextPart"},
		{Err: "io.EOF", Fun: "(*mime/multipart.Reader).NextRawPart"},
		{Err: "mime/multipart.ErrMessageTooLarge", Fun: "(*mime/multipart.Reader).ReadForm"},
	})
}

func allowedMapAppend(ap []AllowPair) {
	for _, pair := range ap {
		if _, ok := allowedErrorsMap[pair.Err]; !ok {
			allowedErrorsMap[pair.Err] = make(map[string]struct{})
		}
		allowedErrorsMap[pair.Err][pair.Fun] = struct{}{}
	}
}

var allowedErrorWildcards = []AllowPair{
	// pkg/syscall
	{Err: "syscall.E", Fun: "syscall."},
	// golang.org/x/sys/unix
	{Err: "golang.org/x/sys/unix.E", Fun: "golang.org/x/sys/unix."},
}

func allowedWildcardAppend(ap []AllowPair) {
	allowedErrorWildcards = append(allowedErrorWildcards, ap...)
}

func isAllowedErrAndFunc(err, fun string) bool {
	if allowedFuncs, allowErr := allowedErrorsMap[err]; allowErr {
		if _, allow := allowedFuncs[fun]; allow {
			return true
		}
	}

	for _, allow := range allowedErrorWildcards {
		if strings.HasPrefix(fun, allow.Fun) && strings.HasPrefix(err, allow.Err) {
			return true
		}
	}

	return false
}

func isAllowedErrorComparison(pass *TypesInfoExt, a, b ast.Expr) bool {
	var errName string // `<package>.<name>`, e.g. `io.EOF`
	var callExprs []*ast.CallExpr

	// Figure out which half of the expression is the returned error and which
	// half is the presumed error declaration.
	for _, expr := range []ast.Expr{a, b} {
		switch t := expr.(type) {
		case *ast.SelectorExpr:
			// A selector which we assume refers to a staticaly declared error
			// in a package.
			errName = selectorToString(pass, t)
		case *ast.Ident:
			// Identifier, most likely to be the `err` variable or whatever
			// produces it.
			callExprs = assigningCallExprs(pass, t, map[types.Object]bool{})
		case *ast.CallExpr:
			callExprs = append(callExprs, t)
		}
	}

	// Unimplemented or not sure, disallow the expression.
	if errName == "" || len(callExprs) == 0 {
		return false
	}

	// Map call expressions to the function name format of the allow list.
	functionNames := make([]string, len(callExprs))
	for i, callExpr := range callExprs {
		functionSelector, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok {
			// If the function is not a selector it is not an Std function that is
			// allowed.
			return false
		}
		if sel, ok := pass.TypesInfo.Selections[functionSelector]; ok {
			functionNames[i] = fmt.Sprintf("(%s).%s", sel.Recv(), sel.Obj().Name())
		} else {
			// If there is no selection, assume it is a package.
			functionNames[i] = selectorToString(pass, callExpr.Fun.(*ast.SelectorExpr))
		}
	}

	// All assignments done must be allowed.
	for _, funcName := range functionNames {
		if !isAllowedErrAndFunc(errName, funcName) {
			return false
		}
	}
	return true
}

// assigningCallExprs finds all *ast.CallExpr nodes that are part of an
// *ast.AssignStmt that assign to the subject identifier.
func assigningCallExprs(pass *TypesInfoExt, subject *ast.Ident, visitedObjects map[types.Object]bool) []*ast.CallExpr {
	if subject.Obj == nil {
		return nil
	}

	// Find other identifiers that reference this same object.
	sobj := pass.TypesInfo.ObjectOf(subject)

	if visitedObjects[sobj] {
		return nil
	}
	visitedObjects[sobj] = true

	// Make sure to exclude the subject identifier as it will cause an infinite recursion and is
	// being used in a read operation anyway.
	identifiers := []*ast.Ident{}
	for _, ident := range pass.IdentifiersForObject[sobj] {
		if subject.Pos() != ident.Pos() {
			identifiers = append(identifiers, ident)
		}
	}

	// Find out whether the identifiers are part of an assignment statement.
	var callExprs []*ast.CallExpr
	for _, ident := range identifiers {
		parent := pass.NodeParent[ident]
		switch declT := parent.(type) {
		case *ast.AssignStmt:
			// The identifier is LHS of an assignment.
			assignment := declT

			assigningExpr := assignment.Rhs[0]
			// If the assignment is comprised of multiple expressions, find out
			// which RHS expression we should use by finding its index in the LHS.
			if len(assignment.Lhs) == len(assignment.Rhs) {
				for i, lhs := range assignment.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && subject.Name == ident.Name {
						assigningExpr = assignment.Rhs[i]
						break
					}
				}
			}

			switch assignT := assigningExpr.(type) {
			case *ast.CallExpr:
				// Found the function call.
				callExprs = append(callExprs, assignT)
			case *ast.Ident:
				// Skip assignments here the RHS points to the same object as the subject.
				if assignT.Obj == subject.Obj {
					continue
				}
				// The subject was the result of assigning from another identifier.
				callExprs = append(callExprs, assigningCallExprs(pass, assignT, visitedObjects)...)
			default:
				// TODO: inconclusive?
			}
		}
	}
	return callExprs
}

func selectorToString(pass *TypesInfoExt, selExpr *ast.SelectorExpr) string {
	o := pass.TypesInfo.Uses[selExpr.Sel]
	return fmt.Sprintf("%s.%s", o.Pkg().Path(), o.Name())
}
