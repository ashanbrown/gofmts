// formatter reformats strings
package gofmts

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"io"
	"strings"
	"unicode"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/jackc/sqlfmt"
	"github.com/pkg/errors"
	"github.com/tidwall/pretty"
)

const tabWidth = 8

type Issue interface {
	Details() string
	Position() token.Position
	String() string
}

type IssueWithReplacement interface {
	Issue
	Replacement() string
	Length() int
}

type Formatter struct {
	applyReplacements bool
}

const directivePrefix = "gofmts:"

func NewFormatter() *Formatter {
	return &Formatter{}
}

func FormatFile(fset *token.FileSet, file *ast.File) ([]Issue, error) {
	fmtr := NewFormatter()
	fmtr.applyReplacements = true
	return fmtr.Run(fset, file)
}

type FormatIssue struct {
	directive   string
	position    token.Position
	end         token.Position
	replacement string
}

func (i FormatIssue) Details() string {
	return fmt.Sprintf("%s formatting differs", i.directive)
}

func (i FormatIssue) Position() token.Position {
	return i.position
}

func (i FormatIssue) Length() int {
	return i.end.Offset - i.position.Offset
}

func (i FormatIssue) String() string { return toString(i) }

func (i FormatIssue) Replacement() string { return i.replacement }

func toString(i Issue) string {
	return fmt.Sprintf("%s at %s", i.Details(), i.Position())
}

type UnusedDirective struct {
	name     string
	position token.Position
}

func (i UnusedDirective) Details() string {
	return fmt.Sprintf("unused directive `%s%s`", directivePrefix, i.name)
}

func (i UnusedDirective) Position() token.Position {
	return i.position
}

func (i UnusedDirective) String() string { return toString(i) }

type UnknownDirective struct {
	directive string
	position  token.Position
}

func (i UnknownDirective) Details() string {
	return fmt.Sprintf("unknown directive `%s%s`", directivePrefix, i.directive)
}

func (i UnknownDirective) Position() token.Position {
	return i.position
}

func (i UnknownDirective) String() string { return toString(i) }

type FailedDirective struct {
	directive string
	position  token.Position
	error     error
}

func (i FailedDirective) Details() string {
	return fmt.Sprintf("failed directive %q: %s", i.directive, i.error)
}

func (i FailedDirective) Position() token.Position {
	return i.position
}

func (i FailedDirective) String() string { return toString(i) }

type formatVisitor struct {
	decorator       *decorator.Decorator
	directivesByPos map[token.Pos]string
	fset            *token.FileSet
	issues          []Issue
	issuesByNode    map[dst.Node]Issue
	prevNode        dst.Node
}

func (f *Formatter) Run(fset *token.FileSet, files ...*ast.File) ([]Issue, error) {
	var issues []Issue // nolint:prealloc // don't know how many there will be
	for _, file := range files {
		dcrtr := decorator.NewDecorator(fset)
		dstFile, err := dcrtr.DecorateFile(file)
		if err != nil {
			return nil, errors.Wrapf(err, "decorate failed")
		}

		directivesByPos := make(map[token.Pos]string) // nolint:prealloc // don't know how many there will be
		issuesByNode := make(map[dst.Node]Issue)      // nolint:prealloc // don't know how many there will be
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if comment.Text[1] == '*' { // only allow directives on //-style comments
					continue
				}

				if strings.HasPrefix(comment.Text[2:], directivePrefix) {
					// ignore sort directives
					if strings.HasPrefix(comment.Text[2:], directivePrefix+"sort") {
						continue
					}
					parts := strings.SplitN(comment.Text[2:], ":", 2)
					directivesByPos[comment.End()] = strings.TrimRightFunc(parts[1], unicode.IsSpace)
				}
			}
		}
		visitor := formatVisitor{
			decorator:       dcrtr,
			directivesByPos: directivesByPos,
			issuesByNode:    issuesByNode,
			fset:            fset,
		}
		dst.Walk(&visitor, dstFile)
		issues = append(issues, visitor.issues...)
		for pos, d := range directivesByPos {
			issues = append(issues, UnusedDirective{name: d, position: fset.Position(pos)})
		}

		// apply replacements
		if f.applyReplacements && len(visitor.issues) > 0 {
			dstutil.Apply(dstFile, nil, func(c *dstutil.Cursor) bool {
				switch node := c.Node().(type) {
				case *dst.BasicLit:
					issue, exists := issuesByNode[c.Node()].(IssueWithReplacement)
					if !exists {
						break
					}
					replacementNode := dst.Clone(node).(*dst.BasicLit)
					replacementNode.Value = issue.Replacement()
					c.Replace(replacementNode)
				}
				return true
			})

			restorer := decorator.NewRestorer()
			restorer.Fset = fset
			af, err := restorer.RestoreFile(dstFile)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to reformat node")
			}
			*file = *af
		}
	}
	return issues, nil
}

func (v *formatVisitor) Visit(node dst.Node) dst.Visitor {
ParseNode:
	switch node := node.(type) {
	case *dst.BasicLit:
		if node.Kind == token.STRING {
			astNode := v.decorator.Ast.Nodes[node]
			closestDirectivePos, closestDirective := findClosestDirective(v.fset, v.directivesByPos, astNode, false)
			if !closestDirectivePos.IsValid() {
				break
			}
			delete(v.directivesByPos, closestDirectivePos) // consume directive
			value := node.Value[1 : len(node.Value)-1]
			var newValue string
			var err error
			switch closestDirective {
			case "json":
				newValue, err = formatJson(value)
			case "mysql", "postgresql", "sql":
				newValue, err = formatSql(value)
			case "go":
				newValue, err = formatGo(value)
			default:
				v.issues = append(v.issues, UnknownDirective{
					directive: closestDirective,
					position:  v.fset.Position(closestDirectivePos),
				})
				break ParseNode
			}
			if err != nil {
				v.issues = append(v.issues, FailedDirective{
					directive: closestDirective,
					position:  v.fset.Position(closestDirectivePos),
					error:     err,
				})
				break
			}

			isMultiline := strings.Contains(newValue, "\n") || v.fset.Position(astNode.Pos()).Line != v.fset.Position(astNode.End()).Line
			if isMultiline && node.Value[0] != '`' {
				issue := FailedDirective{
					directive: closestDirective,
					position:  v.fset.Position(astNode.Pos()),
					error:     errors.New("reformatted string will be multiline and must be quoted using backticks"),
				}
				v.issues = append(v.issues, issue)
				break
			}

			position := v.fset.Position(astNode.Pos())

			replacementBuf := new(bytes.Buffer)
			_, _ = io.WriteString(replacementBuf, node.Value[0:1])
			if isMultiline {
				// start a new line so that tabs and spaces line up (because not all editors use the same tab width)
				_, _ = io.WriteString(replacementBuf, "\n")

				// if we're on a new line, assume that we're indented with tabs
				assumeTabs := false
				if position.Line != v.fset.Position(v.decorator.Ast.Nodes[v.prevNode].Pos()).Line {
					assumeTabs = true
				}

				// the indent column is basically a WAG because we don't know what's a tab and what's a space
				columnByteOffset := v.fset.Position(astNode.Pos()).Column
				indentSpaces := columnByteOffset
				if assumeTabs {
					indentSpaces = columnByteOffset*tabWidth + 1
				}

				iw := NewIndentWriter(replacementBuf, indentSpaces, tabWidth /* tab width */)
				_ = iw.WriteString(newValue, false)
				_ = iw.WriteString(node.Value[len(node.Value)-1:], true)
			} else {
				_, _ = io.WriteString(replacementBuf, newValue)
				_, _ = io.WriteString(replacementBuf, node.Value[len(node.Value)-1:])
			}

			// continue to next node if there are no changes
			if replacementBuf.String() == node.Value {
				break
			}

			issue := FormatIssue{
				directive:   closestDirective,
				position:    position,
				end:         v.fset.Position(astNode.End()),
				replacement: replacementBuf.String(),
			}
			v.issuesByNode[node] = issue
			v.issues = append(v.issues, issue)
		}
	}
	if node != nil {
		v.prevNode = node
	}
	return v
}

func findClosestDirective(fset *token.FileSet, directivesByPos map[token.Pos]string, node ast.Node, ignoreInline bool) (pos token.Pos, directive string) {
	pos = token.NoPos
	for p, d := range directivesByPos {
		directiveStartLine := fset.Position(p).Line
		var applies bool
		if ignoreInline {
			nodeStartLine := fset.Position(node.Pos()).Line
			applies = directiveStartLine <= nodeStartLine
		} else {
			nodeEndLine := fset.Position(node.End()).Line
			applies = directiveStartLine <= nodeEndLine
		}
		if applies && p > pos {
			pos = p
			directive = d
		}
	}
	return pos, directive
}

func formatJson(value string) (string, error) {
	if valid := json.Valid([]byte(value)); !valid {
		return "", errors.New("json is not valid")
	}
	newValue := pretty.Pretty([]byte(value))
	return string(newValue), nil
}

func formatSql(value string) (string, error) {
	lexer := sqlfmt.NewSqlLexer(value)
	stmt, err := sqlfmt.Parse(lexer)
	if err != nil {
		return "", errors.Wrapf(err, "unable to parse sql")
	}

	outBuf := new(bytes.Buffer)
	r := sqlfmt.NewTextRenderer(outBuf)
	r.UpperCase = true
	stmt.RenderTo(r)
	return outBuf.String(), nil
}

func formatGo(value string) (string, error) {
	formatted, err := format.Source([]byte(value))
	if err != nil {
		return "", errors.Wrapf(err, "unable to format go code")
	}
	return string(formatted), nil
}

type indentWriter struct {
	w      io.Writer
	indent string
}

func NewIndentWriter(w io.Writer, indentColumn, tabWidth int) indentWriter {
	iw := indentWriter{
		w: w,
		indent: strings.Repeat("	", indentColumn/tabWidth) + strings.Repeat(" ", indentColumn%tabWidth),
	}
	return iw
}

func (w indentWriter) WriteString(p string, skipNewline bool) error {
	scanner := bufio.NewScanner(bytes.NewBufferString(p))
	for scanner.Scan() {
		if _, err := io.WriteString(w.w, w.indent); err != nil {
			return err
		}
		if _, err := w.w.Write(scanner.Bytes()); err != nil {
			return err
		}
		if !skipNewline {
			if _, err := io.WriteString(w.w, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}
