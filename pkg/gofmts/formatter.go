// formatter reformats strings
package gofmts

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
	"golang.org/x/tools/go/ast/astutil"
)

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
}

const directivePrefix = "gofmts:"

func NewFormatter() *Formatter {
	return &Formatter{}
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

type visitor struct {
	directivesByPos map[token.Pos]string
	fset            *token.FileSet
	issues          []Issue
	issuesByPos     map[token.Pos]Issue
}

func Rewrite(fset *token.FileSet, nodes ...ast.Node) error {
	f := Formatter{}
	issues, err := f.Run(fset, nodes...)
	if err != nil {
		return err
	}
	for _, i := range issues {
		if _, hasReplacement := i.(IssueWithReplacement); !hasReplacement {
			return errors.New(i.Details())
		}
	}
	return nil
}

// nodes may be modified by this method
func (f *Formatter) Run(fset *token.FileSet, nodes ...ast.Node) ([]Issue, error) {
	var issues []Issue // nolint:prealloc // don't know how many there will be
	for _, node := range nodes {
		directivesByPos := make(map[token.Pos]string) // nolint:prealloc // don't know how many there will be
		issuesByPos := make(map[token.Pos]Issue)      // nolint:prealloc // don't know how many there will be
		switch node := node.(type) {
		case *ast.File:
			for _, group := range node.Comments {
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
		}
		visitor := visitor{
			directivesByPos: directivesByPos,
			issuesByPos:     issuesByPos,
			fset:            fset,
		}
		ast.Walk(&visitor, node)
		issues = append(issues, visitor.issues...)
		for pos, d := range directivesByPos {
			issues = append(issues, UnusedDirective{name: d, position: fset.Position(pos)})
		}

		// apply replacements
		astutil.Apply(node, nil, func(c *astutil.Cursor) bool {
			switch node := c.Node().(type) {
			case *ast.BasicLit:
				issue, exists := issuesByPos[node.Pos()].(IssueWithReplacement)
				if !exists {
					break
				}
				replacementNode := &ast.BasicLit{
					Kind:  token.STRING,
					Value: issue.Replacement(),
				}
				c.Replace(replacementNode)
			}
			return true
		})

		srtr := Sorter{}
		sortIssues, err := srtr.run(fset, node)
		if err != nil {
			return nil, errors.Wrapf(err, "sort failed")
		}
		issues = append(issues, sortIssues...)
	}
	return issues, nil
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
ParseNode:
	switch node := node.(type) {
	case *ast.BasicLit:
		if node.Kind == token.STRING {
			closestDirectivePos, closestDirective := findClosestDirective(v.fset, v.directivesByPos, node, false)
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
			issue := FormatIssue{
				directive:   closestDirective,
				position:    v.fset.Position(node.Pos()),
				end:         v.fset.Position(node.End()),
				replacement: replacementBuf.String(),
			}
			v.issuesByPos[node.Pos()] = issue
			v.issues = append(v.issues, issue)
		}
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
