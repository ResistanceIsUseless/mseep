package fuzzy

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
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

// SelectBest returns the best match, prompting user if there are multiple good matches
func SelectBest(query string, idx []Index, assumeYes bool) (*Index, error) {
	candidates := Candidates(query, idx)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no match for %q", query)
	}

	// If only one candidate or assuming yes, return the best
	if len(candidates) == 1 || assumeYes {
		return &candidates[0].Index, nil
	}

	// Check if there are multiple candidates with similar scores (ambiguous)
	bestScore := candidates[0].Score
	ambiguousThreshold := 5 // If scores are within 5 points, consider ambiguous
	
	var goodCandidates []Result
	for _, c := range candidates {
		if bestScore-c.Score <= ambiguousThreshold {
			goodCandidates = append(goodCandidates, c)
		} else {
			break // Scores are sorted descending, so we can stop
		}
	}

	// If there's a clear winner, return it
	if len(goodCandidates) == 1 {
		return &goodCandidates[0].Index, nil
	}

	// Multiple ambiguous matches - prompt user
	fmt.Printf("Multiple matches found for %q:\n", query)
	for i, c := range goodCandidates {
		fmt.Printf("  %d) %s", i+1, c.Index.Name)
		
		// Show additional context
		var details []string
		if len(c.Index.Aliases) > 0 {
			details = append(details, fmt.Sprintf("aliases: %s", strings.Join(c.Index.Aliases, ", ")))
		}
		if len(c.Index.Tags) > 0 {
			details = append(details, fmt.Sprintf("tags: %s", strings.Join(c.Index.Tags, ", ")))
		}
		if len(details) > 0 {
			fmt.Printf(" (%s)", strings.Join(details, ", "))
		}
		fmt.Printf(" [score: %d]\n", c.Score)
	}
	
	fmt.Print("Select option [1]: ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read selection: %w", err)
	}
	
	response = strings.TrimSpace(response)
	if response == "" {
		response = "1" // Default to first option
	}
	
	selection, err := strconv.Atoi(response)
	if err != nil || selection < 1 || selection > len(goodCandidates) {
		return nil, fmt.Errorf("invalid selection %q", response)
	}
	
	return &goodCandidates[selection-1].Index, nil
}
