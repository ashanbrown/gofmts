// formatter reformats strings
package formatter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"strings"
	"unicode"

	"github.com/jackc/sqlfmt"
	"github.com/pkg/errors"
	"github.com/tidwall/pretty"
)

type Issue interface {
	Details() string
	Position() token.Position
	String() string
	Replacement() *string
}

type Formatter struct {
}

const directivePrefix = "gofmts:"

func NewFormatter() *Formatter {
	return &Formatter{}
}

type FormatIssue struct {
	directive   string
	position    token.Position
	replacement string
}

func (i FormatIssue) Details() string {
	return fmt.Sprintf("%s formatting differs", i.directive)
}

func (i FormatIssue) Position() token.Position {
	return i.position
}

func (i FormatIssue) String() string { return toString(i) }

func (i FormatIssue) Replacement() *string { return &i.replacement }

func toString(i Issue) string {
	return fmt.Sprintf("%s at %s", i.Details(), i.Position())
}

type UnusedDirective struct {
	name     string
	position token.Position
}

func (i UnusedDirective) Details() string {
	return fmt.Sprintf("unused directive `%s:%s`", directivePrefix, i.name)
}

func (i UnusedDirective) Position() token.Position {
	return i.position
}

func (i UnusedDirective) String() string { return toString(i) }

func (i UnusedDirective) Replacement() *string { return nil }

type UnknownDirective struct {
	directive string
	position  token.Position
}

func (i UnknownDirective) Details() string {
	return fmt.Sprintf("unknown directive `%s:%s`", directivePrefix, i.directive)
}

func (i UnknownDirective) Position() token.Position {
	return i.position
}

func (i UnknownDirective) String() string { return toString(i) }

func (i UnknownDirective) Replacement() *string { return nil }

type FailedDirective struct {
	directive string
	position  token.Position
	error     error
}

func (i FailedDirective) Details() string {
	return fmt.Sprintf("failed directive `%s`: %s", i.directive, i.error)
}

func (i FailedDirective) Position() token.Position {
	return i.position
}

func (i FailedDirective) String() string { return toString(i) }

func (i FailedDirective) Replacement() *string { return nil }

type visitor struct {
	directivesByPos map[token.Pos]string
	fset            *token.FileSet
	issues          []Issue
}

func (f *Formatter) Run(fset *token.FileSet, nodes ...ast.Node) ([]Issue, error) {
	directivesByPos := make(map[token.Pos]string) // nolint:prealloc // don't know how many there will be
	var issues []Issue                            // nolint:prealloc // don't know how many there will be
	for _, node := range nodes {
		switch node := node.(type) {
		case *ast.File:
			for _, group := range node.Comments {
				for _, comment := range group.List {
					if comment.Text[1] == '*' { // only allow directives on //-style comments
						continue
					}

					if strings.HasPrefix(comment.Text[2:], directivePrefix) {
						parts := strings.SplitN(comment.Text[2:], ":", 2)
						directivesByPos[comment.End()] = strings.TrimRightFunc(parts[1], unicode.IsSpace)
					}
				}
			}
		}
		visitor := visitor{
			directivesByPos: directivesByPos,
			fset:            fset,
		}
		ast.Walk(&visitor, node)
		issues = append(issues, visitor.issues...)
		for pos, d := range directivesByPos {
			issues = append(issues, UnusedDirective{name: d, position: fset.Position(pos)})
		}
	}
	return issues, nil
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	switch node := node.(type) {
	case *ast.BasicLit:
		if node.Kind == token.STRING {
			closestDirectivePos, closestDirective := findClosestDirective(v.directivesByPos, node.Pos())
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
			default:
				v.issues = append(v.issues, UnknownDirective{
					directive: closestDirective,
					position:  v.fset.Position(closestDirectivePos),
				})
			}
			if err != nil {
				v.issues = append(v.issues, FailedDirective{
					directive: closestDirective,
					position:  v.fset.Position(closestDirectivePos),
					error:     err,
				})
			}
			var replacementBuf bytes.Buffer
			replacementBuf.WriteByte(node.Value[0])
			isMultiline := strings.Contains(newValue, "\n") || v.fset.Position(node.Pos()).Line != v.fset.Position(node.End()).Line
			if isMultiline {
				indentColumn := v.fset.Position(node.Pos()).Column
				indent := strings.Repeat("\t", indentColumn/8) + strings.Repeat(" ", indentColumn%8)
				_, _ = replacementBuf.WriteString("\n")
				lines := strings.Split(newValue, "\n")
				for i, line := range lines {
					replacementBuf.WriteString(indent)
					_, _ = replacementBuf.WriteString(line)
					if i < len(lines)-1 {
						replacementBuf.WriteString("\n")
					}
				}
			} else {
				replacementBuf.WriteString(newValue)
			}
			replacementBuf.WriteByte(node.Value[len(node.Value)-1])
			v.issues = append(v.issues, FormatIssue{
				directive:   closestDirective,
				position:    v.fset.Position(node.Pos()),
				replacement: replacementBuf.String(),
			})
		}
	}
	return v
}

func findClosestDirective(directivesByPos map[token.Pos]string, nodePos token.Pos) (pos token.Pos, directive string) {
	pos = token.NoPos
	for p, d := range directivesByPos {
		if p < nodePos && p > pos {
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
	stmt.RenderTo(r)
	return outBuf.String(), nil
}
