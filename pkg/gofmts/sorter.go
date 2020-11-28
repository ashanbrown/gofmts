package gofmts

import (
	"bufio"
	"bytes"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"sort"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/pkg/errors"
)

type Sorter struct {
	skipReplacements bool // don't generate the replacement strings
}

type SortIssue struct {
	directive   string
	position    token.Position
	end         token.Position
	replacement string
}

func (i SortIssue) Details() string {
	return "block is unsorted"
}

func (i SortIssue) Position() token.Position {
	return i.position
}

func (i SortIssue) Length() int {
	return i.end.Offset - i.position.Offset
}

func (i SortIssue) String() string { return toString(i) }

func (i SortIssue) Replacement() string { return i.replacement }

type sortGroup struct {
	directive    string
	directivePos token.Pos
	nodes        []dst.Node
}

func (g *sortGroup) endPos(dcrtr *decorator.Decorator) token.Pos {
	if len(g.nodes) > 0 {
		return dcrtr.Ast.Nodes[g.nodes[len(g.nodes)-1]].End()
	}
	return g.directivePos
}

func (g *sortGroup) startPos(dcrtr *decorator.Decorator) token.Pos {
	return dcrtr.Ast.Nodes[g.nodes[0]].Pos()
}

type sortVisitor struct {
	decorator       *decorator.Decorator
	directivesByPos map[token.Pos]string
	sortGroups      []*sortGroup
	fset            *token.FileSet
	activeSortGroup *sortGroup
}

// nodes may be modified by this method
func (s *Sorter) run(fset *token.FileSet, files ...*ast.File) (issues []Issue, _ error) {
	for _, file := range files {
		directivesByPos := make(map[token.Pos]string) // nolint:prealloc // don't know how many there will be
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if comment.Text[1] == '*' { // only allow directives on //-style comments
					continue
				}

				if strings.HasPrefix(comment.Text[2:], directivePrefix+"sort") {
					directivesByPos[comment.End()] = "sort"
				}
			}
		}
		dcrtr := decorator.NewDecorator(fset)
		pos := func(n dst.Node) token.Pos {
			return dcrtr.Ast.Nodes[n].Pos()
		}
		visitor := &sortVisitor{
			decorator:       dcrtr,
			directivesByPos: directivesByPos,
			fset:            fset,
		}
		dstFile, err := dcrtr.DecorateFile(file)
		if err != nil {
			return nil, errors.Wrapf(err, "decorate failed")
		}
		dst.Walk(visitor, dstFile)

		replacementNodes := make(map[dst.Node]dst.Node)

		// create issues from the sort groups
		for _, g := range visitor.sortGroups {
			sortedNodes := make([]dst.Node, len(g.nodes))
			copy(sortedNodes, g.nodes)
			sort.Sort(sortNodesLexicographically{nodes: sortedNodes, fset: fset, decorator: dcrtr})
			hasChanges := false
			for i, n := range g.nodes {
				// if we've moved this node
				if pos(n) != pos(sortedNodes[i]) {
					// clone the node taking this spot
					repl := dst.Clone(sortedNodes[i])
					if i == 0 {
						// move the "preamble" (including the sort directive) to the new start
						preamble := n.Decorations().Start.All()
						n.Decorations().Start.Clear()
						repl.Decorations().Start.Prepend(preamble...)
					}
					replacementNodes[n] = repl
					hasChanges = true
				}
			}
			if !hasChanges {
				continue // no changes
			}
			issue := SortIssue{
				directive: g.directive,
				position:  fset.Position(g.startPos(dcrtr)),
				end:       fset.Position(g.endPos(dcrtr)),
			}
			issues = append(issues, issue)
		}

		for pos := range directivesByPos {
			issues = append(issues, UnusedDirective{name: "sort", position: fset.Position(pos)})
		}

		dstutil.Apply(dstFile, nil, func(cursor *dstutil.Cursor) bool {
			if cursor.Node() == nil {
				return true
			} else if repl := replacementNodes[cursor.Node()]; repl != nil {
				cursor.Replace(repl)
			}
			return true
		})

		if len(replacementNodes) > 0 {
			restorer := decorator.NewRestorer()
			restorer.Fset = dcrtr.Fset
			af, err := restorer.RestoreFile(dstFile)
			if err != nil {
				return nil, errors.Wrap(err, "dst restore failed")
			}
			*file = *af

			if !s.skipReplacements {
				fbuf := new(bytes.Buffer)
				err := format.Node(fbuf, restorer.Fset, file)
				if err != nil {
					return nil, errors.Wrap(err, "dst restore failed")
				}
				lines := readLines(fbuf)
				for i, issue := range issues {
					if s, ok := issue.(SortIssue); ok {
						s.replacement = strings.Join(lines[s.position.Line-1:s.end.Line], "")
						issues[i] = s
					}
				}
			}
		}
	}
	return issues, nil
}

func readLines(fbuf *bytes.Buffer) []string {
	var lines []string
	scanner := bufio.NewScanner(fbuf)
	for scanner.Scan() {
		lines = append(lines, scanner.Text()+"\n")
	}
	return lines
}

func (v *sortVisitor) Visit(node dst.Node) dst.Visitor {
	if node == nil {
		return nil
	}

	if v.activeSortGroup == nil {
		directivePos, directive := findClosestDirective(v.fset, v.directivesByPos, v.decorator.Ast.Nodes[node], true)
		pos := v.decorator.Ast.Nodes[node].Pos()
		directiveLine := v.fset.Position(directivePos).Line
		nodeLine := v.fset.Position(pos).Line
		if directivePos != token.NoPos && nodeLine <= directiveLine+len(node.Decorations().Start) {
			v.activeSortGroup = &sortGroup{
				directive:    directive,
				directivePos: directivePos,
				nodes:        []dst.Node{node},
			}
			v.sortGroups = append(v.sortGroups, v.activeSortGroup)
			delete(v.directivesByPos, directivePos)
			return nil // skip children now that we have a sort group
		}
		return v // continue to visit children if we haven't created a sort group yet
	}

	// node is not part of group, so close the group
	if node.Decorations().Before != dst.NewLine {
		v.activeSortGroup = nil
		return v // walk children
	}

	v.activeSortGroup.nodes = append(v.activeSortGroup.nodes, node)
	return nil // skip children since this node and its children are in current group
}

type sortNodesLexicographically struct {
	nodes     []dst.Node
	fset      *token.FileSet
	decorator *decorator.Decorator
}

func (s sortNodesLexicographically) Len() int {
	return len(s.nodes)
}

func (s sortNodesLexicographically) Less(a, b int) bool {
	return s.renderNode(s.nodes[a]) < s.renderNode(s.nodes[b])
}

func (s sortNodesLexicographically) Swap(a, b int) {
	s.nodes[a], s.nodes[b] = s.nodes[b], s.nodes[a]
}

func (s sortNodesLexicographically) renderNode(node dst.Node) string {
	astNode := stripComments(s.decorator.Ast.Nodes[node])

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, s.fset, stripComments(astNode)); err != nil {
		panic(err)
	}
	return strings.TrimSpace(buf.String())
}

func stripComments(n ast.Node) ast.Node {
	switch n := n.(type) {
	case *ast.Field:
		v := new(ast.Field)
		*v = *n
		v.Doc = nil
		v.Comment = nil
		return v
	case *ast.ImportSpec:
		v := new(ast.ImportSpec)
		*v = *n
		v.Doc = nil
		v.Comment = nil
		return v
	case *ast.ValueSpec:
		v := new(ast.ValueSpec)
		*v = *n
		v.Doc = nil
		v.Comment = nil
		return v
	case *ast.TypeSpec:
		v := new(ast.TypeSpec)
		*v = *n
		v.Doc = nil
		v.Comment = nil
		return v
	case *ast.GenDecl:
		v := new(ast.GenDecl)
		*v = *n
		v.Doc = nil
		for i, s := range n.Specs {
			v.Specs[i] = stripComments(s).(ast.Spec)
		}
		return v
	case *ast.FuncDecl:
		v := new(ast.FuncDecl)
		*v = *n
		return v
	}
	return n
}
