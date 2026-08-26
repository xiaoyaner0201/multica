// Command discover_email_behavior enumerates the behavior surface of the
// outbound-email producers for HD-48.
//
// It is an AST walker over every non-vendor Go file under the given root. It is
// NOT type-checked: call targets are matched by selector/identifier NAME, not by
// resolved method set. That limitation is deliberate (golang.org/x/tools is not
// in this environment's module cache, so go/packages is unavailable offline) and
// is the reason the resulting inventory is declared maturity D1 rather than D2.
//
// Usage:
//
//	go run ./discover_email_behavior.go -root <repo>/server
//
// Output is deterministic: every section is sorted by file path then line.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// producers + internal helpers whose call sites define the changed-behavior surface.
var targetCalls = map[string]bool{
	"SendVerificationCode":  true,
	"SendInvitationEmail":   true,
	"sendSMTP":              true,
	"buildInvitationParams": true,
	"sanitizeSubjectField":  true,
	"NewEmailService":       true,
	"resolveFromEmail":      true,
	"openSMTPClient":        true,
}

// terminal side-effect sinks: anything here actually emits bytes to a recipient
// (or to operator stdout, which is the DEV recipient).
var sinkCalls = map[string]bool{
	"Send":    true, // resend Emails.Send
	"Printf":  true, // DEV stdout
	"Println": true, // DEV stdout
	"Fprintf": true, // SMTP DATA writer
	"Mail":    true, // SMTP MAIL FROM
	"Rcpt":    true, // SMTP RCPT TO
	"Data":    true, // SMTP DATA
}

// escaping / encoding models applied to recipient-visible text.
var escapeCalls = map[string]bool{
	"EscapeString":         true, // html.EscapeString  -> body model
	"sanitizeSubjectField": true, // control-strip+cap   -> subject model
	"Encode":               true, // mime.QEncoding.Encode -> subject transport model
	"NewWriter":            true, // quotedprintable.NewWriter -> body transport model
}

type site struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Callee    string `json:"callee"`
	Enclosing string `json:"enclosing"`
	InGo      bool   `json:"in_go_stmt"`
	InDefer   bool   `json:"in_defer_stmt"`
}

type literal struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Owner string `json:"enclosing"`
	Head  string `json:"head"`
}

type report struct {
	Root          string    `json:"root"`
	FilesParsed   int       `json:"files_parsed"`
	ProducerCalls []site    `json:"producer_and_helper_call_sites"`
	Sinks         []site    `json:"sink_call_sites_in_email_go"`
	Escapes       []site    `json:"escape_or_encode_call_sites_in_email_go"`
	Literals      []literal `json:"recipient_visible_string_literals_in_email_go"`
	EnvKeys       []string  `json:"env_keys_read_in_email_go"`
	Unresolved    []string  `json:"unresolved_notes"`
}

func calleeName(e ast.Expr) string {
	switch f := e.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	case *ast.IndexExpr:
		return calleeName(f.X)
	}
	return ""
}

func main() {
	root := flag.String("root", "server", "root directory to walk")
	focus := flag.String("focus", "internal/service/email.go", "file (suffix match) for sink/escape/literal sections")
	prefix := flag.String("prefix", "", "path prefix trimmed from every emitted filename, so output is repo-relative and location-independent")
	flag.Parse()
	trim := func(p string) string {
		p = filepath.ToSlash(p)
		if *prefix != "" {
			p = strings.TrimPrefix(p, filepath.ToSlash(*prefix))
		}
		return strings.TrimPrefix(p, "/")
	}

	rep := report{Root: trim(*root)}
	fset := token.NewFileSet()

	var files []string
	err := filepath.Walk(*root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == "vendor" || base == "node_modules" || strings.HasPrefix(base, ".") && base != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "walk:", err)
		os.Exit(1)
	}
	sort.Strings(files)
	rep.FilesParsed = len(files)

	for _, p := range files {
		f, perr := parser.ParseFile(fset, p, nil, parser.ParseComments)
		if perr != nil {
			rep.Unresolved = append(rep.Unresolved, "parse-error "+p+": "+perr.Error())
			continue
		}
		isFocus := strings.HasSuffix(filepath.ToSlash(p), *focus)

		// Track enclosing function and go/defer nesting.
		var enclosing string
		var goDepth, deferDepth int

		var walk func(n ast.Node)
		walk = func(n ast.Node) {
			ast.Inspect(n, func(x ast.Node) bool {
				switch v := x.(type) {
				case *ast.FuncDecl:
					prev := enclosing
					name := v.Name.Name
					if v.Recv != nil && len(v.Recv.List) > 0 {
						name = "(" + exprString(v.Recv.List[0].Type) + ")." + name
					}
					enclosing = name
					if v.Body != nil {
						walk(v.Body)
					}
					enclosing = prev
					return false
				case *ast.GoStmt:
					goDepth++
					walk(v.Call)
					goDepth--
					return false
				case *ast.DeferStmt:
					deferDepth++
					walk(v.Call)
					deferDepth--
					return false
				case *ast.CallExpr:
					name := calleeName(v.Fun)
					pos := fset.Position(v.Pos())
					s := site{
						File:      trim(pos.Filename),
						Line:      pos.Line,
						Callee:    exprString(v.Fun),
						Enclosing: enclosing,
						InGo:      goDepth > 0,
						InDefer:   deferDepth > 0,
					}
					if targetCalls[name] {
						rep.ProducerCalls = append(rep.ProducerCalls, s)
					}
					if isFocus && sinkCalls[name] {
						rep.Sinks = append(rep.Sinks, s)
					}
					if isFocus && escapeCalls[name] {
						rep.Escapes = append(rep.Escapes, s)
					}
					if isFocus && name == "Getenv" && len(v.Args) == 1 {
						if bl, ok := v.Args[0].(*ast.BasicLit); ok && bl.Kind == token.STRING {
							if k, e := strconv.Unquote(bl.Value); e == nil {
								rep.EnvKeys = append(rep.EnvKeys, k)
							}
						}
					}
					return true
				case *ast.BasicLit:
					if isFocus && v.Kind == token.STRING {
						raw, e := strconv.Unquote(v.Value)
						if e != nil {
							raw = strings.Trim(v.Value, "`")
						}
						if looksRecipientVisible(raw) {
							pos := fset.Position(v.Pos())
							rep.Literals = append(rep.Literals, literal{
								File:  trim(pos.Filename),
								Line:  pos.Line,
								Owner: enclosing,
								Head:  headline(raw),
							})
						}
					}
					return true
				}
				return true
			})
		}
		walk(f)
	}

	sortSites(rep.ProducerCalls)
	sortSites(rep.Sinks)
	sortSites(rep.Escapes)
	sort.Slice(rep.Literals, func(i, j int) bool {
		if rep.Literals[i].File != rep.Literals[j].File {
			return rep.Literals[i].File < rep.Literals[j].File
		}
		return rep.Literals[i].Line < rep.Literals[j].Line
	})
	rep.EnvKeys = uniqSorted(rep.EnvKeys)

	rep.Unresolved = append(rep.Unresolved,
		"NAME-BASED AST MATCH ONLY (no go/types): a call through an interface value, a "+
			"function value, or reflect.MethodByName whose selector text differs from the "+
			"target name is NOT reported by this tool.",
		"Templates/assets are not scanned: this tool proves nothing about non-Go files.",
	)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rep); err != nil {
		fmt.Fprintln(os.Stderr, "encode:", err)
		os.Exit(1)
	}
}

func sortSites(s []site) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].File != s[j].File {
			return s[i].File < s[j].File
		}
		return s[i].Line < s[j].Line
	})
}

func uniqSorted(in []string) []string {
	m := map[string]bool{}
	for _, v := range in {
		m[v] = true
	}
	out := make([]string, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// looksRecipientVisible flags literals that can end up in a recipient's inbox or
// on the wire: HTML fragments, MIME headers, and the two hardcoded subjects.
func looksRecipientVisible(s string) bool {
	if len(s) < 4 {
		return false
	}
	needles := []string{"<div", "<h2", "<p", "<a ", "verification code", "invited you to",
		"Content-Type", "Content-Transfer-Encoding", "MIME-Version", "Subject: ", "From: ", "To: ", "[DEV]"}
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func headline(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	s = strings.ReplaceAll(s, "\r", "\\r")
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}

func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return exprString(v.X) + "." + v.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(v.X)
	case *ast.CallExpr:
		return exprString(v.Fun) + "(...)"
	case *ast.IndexExpr:
		return exprString(v.X)
	case *ast.ParenExpr:
		return exprString(v.X)
	}
	return "?"
}
