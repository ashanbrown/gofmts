package gofmts

import (
	"io"
	"io/ioutil"
)

func ApplyReplacements(w io.Writer, r io.Reader, issues []Issue) (unresolvedIssues []Issue, _ error) {
	lastOffset := 0
	for _, i := range issues {
		replacement, ok := i.(IssueWithReplacement)
		if !ok {
			unresolvedIssues = append(unresolvedIssues, replacement)
		}
		if _, err := io.CopyN(w, r, int64(replacement.Position().Offset-lastOffset)); err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(replacement.Replacement())); err != nil {
			return nil, err
		}
		if _, err := io.CopyN(ioutil.Discard, r, int64(replacement.Length())); err != nil {
			return nil, err
		}
		lastOffset = replacement.Position().Offset + replacement.Length()
	}
	// copy the rest
	_, err := io.Copy(w, r)
	return unresolvedIssues, err
}
