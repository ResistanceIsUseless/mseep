package fuzzy

import (
	"testing"
)

func TestScoreOne(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		target   string
		expected int
	}{
		{
			name:     "exact match",
			query:    "github",
			target:   "github",
			expected: 100,
		},
		{
			name:     "case insensitive exact match",
			query:    "GitHub",
			target:   "github",
			expected: 100,
		},
		{
			name:     "contains match",
			query:    "hub",
			target:   "github",
			expected: 77, // 80 - (6-3)
		},
		{
			name:     "no match",
			query:    "xyz",
			target:   "github",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test through the Candidates function since scoreOne is not exported
			idx := []Index{{Name: tt.target}}
			results := Candidates(tt.query, idx)
			
			if tt.expected == 0 {
				if len(results) != 0 {
					t.Errorf("expected no results for query %q, got %d", tt.query, len(results))
				}
			} else {
				if len(results) == 0 {
					t.Fatalf("expected results for query %q, got none", tt.query)
				}
				if results[0].Score != tt.expected {
					t.Errorf("score(%q, %q) = %d, want %d", tt.query, tt.target, results[0].Score, tt.expected)
				}
			}
		})
	}
}

func TestCandidates(t *testing.T) {
	indexes := []Index{
		{Name: "github", Aliases: []string{"gh"}, Tags: []string{"vcs", "code"}},
		{Name: "gitlab", Aliases: []string{"gl"}, Tags: []string{"vcs"}},
		{Name: "obsidian", Aliases: []string{"obs"}, Tags: []string{"notes", "markdown"}},
		{Name: "burp", Aliases: []string{}, Tags: []string{"security", "proxy"}},
	}

	tests := []struct {
		name           string
		query          string
		expectedNames  []string
		expectedScores []int
	}{
		{
			name:           "match by name",
			query:          "github",
			expectedNames:  []string{"github"},
			expectedScores: []int{100},
		},
		{
			name:           "match by alias",
			query:          "gh",
			expectedNames:  []string{"github"},
			expectedScores: []int{100},
		},
		{
			name:           "match by tag",
			query:          "vcs",
			expectedNames:  []string{"github", "gitlab"},
			expectedScores: []int{100, 100},
		},
		{
			name:           "partial match",
			query:          "git",
			expectedNames:  []string{"github", "gitlab"},
			expectedScores: []int{77, 77},
		},
		{
			name:           "no matches",
			query:          "nonexistent",
			expectedNames:  []string{},
			expectedScores: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := Candidates(tt.query, indexes)

			if len(results) != len(tt.expectedNames) {
				t.Fatalf("got %d results, want %d", len(results), len(tt.expectedNames))
			}

			for i, result := range results {
				if result.Index.Name != tt.expectedNames[i] {
					t.Errorf("result[%d].Name = %s, want %s", i, result.Index.Name, tt.expectedNames[i])
				}
				if result.Score != tt.expectedScores[i] {
					t.Errorf("result[%d].Score = %d, want %d", i, result.Score, tt.expectedScores[i])
				}
			}
		})
	}
}

func TestBestMatch(t *testing.T) {
	indexes := []Index{
		{Name: "github", Aliases: []string{"gh"}, Tags: []string{"vcs"}},
		{Name: "gitlab", Aliases: []string{"gl"}, Tags: []string{"vcs"}},
		{Name: "gitea", Aliases: []string{}, Tags: []string{"vcs"}},
	}

	tests := []struct {
		name         string
		query        string
		expectedName string
		expectNil    bool
	}{
		{
			name:         "exact match preferred",
			query:        "github",
			expectedName: "github",
			expectNil:    false,
		},
		{
			name:         "partial match selects any with best score",
			query:        "git",
			expectedName: "", // Any of the matches is acceptable
			expectNil:    false,
		},
		{
			name:         "no match returns nil",
			query:        "xyz",
			expectedName: "",
			expectNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := Candidates(tt.query, indexes)
			
			if tt.expectNil {
				if len(candidates) != 0 {
					t.Errorf("expected no candidates, got %d", len(candidates))
				}
			} else {
				if len(candidates) == 0 {
					t.Fatal("expected candidates, got none")
				}
				if tt.expectedName != "" && candidates[0].Index.Name != tt.expectedName {
					t.Errorf("best match = %s, want %s", candidates[0].Index.Name, tt.expectedName)
				}
			}
		})
	}
}