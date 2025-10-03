package fuzzy

import (
	"sort"
	"strings"
)

type Index struct {
	Name    string
	Aliases []string
	Tags    []string
}

type Result struct {
	Index Index
	Score int
}

func scoreOne(q, s string) int {
	q = strings.ToLower(strings.TrimSpace(q))
	s = strings.ToLower(strings.TrimSpace(s))
	if q == "" || s == "" { return 0 }
	if q == s { return 100 }
	if strings.Contains(s, q) {
		// Prefer shorter candidates slightly
		return 80 - (len(s)-len(q))
	}
	// Token-based: all tokens present earns some score
	toks := strings.Fields(q)
	present := 0
	for _, t := range toks {
		if strings.Contains(s, t) { present++ }
	}
	if present == len(toks) && len(toks) > 0 {
		return 60 - (len(s) - len(q))
	}
	return 0
}

func Candidates(query string, idx []Index) []Result {
	q := strings.TrimSpace(query)
	if q == "" { return nil }

	scores := make([]Result, 0, len(idx))
	for _, it := range idx {
		best := scoreOne(q, it.Name)
		for _, s := range it.Aliases { if sc := scoreOne(q, s); sc > best { best = sc } }
		for _, s := range it.Tags { if sc := scoreOne(q, s); sc > best { best = sc } }
		if best > 0 {
			scores = append(scores, Result{Index: it, Score: best})
		}
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].Score > scores[j].Score })
	return scores
}
