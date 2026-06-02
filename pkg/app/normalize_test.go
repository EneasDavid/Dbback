package app

import "testing"

func TestAuthorDisplayNamePrefersNickname(t *testing.T) {
	tests := []struct {
		name   string
		author string
		want   string
	}{
		{name: "parentheses", author: "Davide Eneas (Davi)", want: "Davi"},
		{name: "double quotes", author: `Maria Silva "Mari"`, want: "Mari"},
		{name: "smart quotes", author: "João Pessoa “Jota”", want: "Jota"},
		{name: "full name fallback", author: "Ana Beatriz Lima", want: "Ana Beatriz Lima"},
		{name: "email fallback", author: "Professor (professor@example.com)", want: "Professor (professor@example.com)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := authorDisplayName(tt.author); got != tt.want {
				t.Fatalf("authorDisplayName(%q) = %q, want %q", tt.author, got, tt.want)
			}
		})
	}
}

func TestScoreToneFromRatioUsesRequestedBands(t *testing.T) {
	tests := []struct {
		ratio float64
		want  string
	}{
		{ratio: 30, want: "score-danger"},
		{ratio: 30.1, want: "score-warning"},
		{ratio: 69.9, want: "score-warning"},
		{ratio: 70, want: "score-success"},
	}

	for _, tt := range tests {
		if got := scoreToneFromRatio(tt.ratio, false); got != tt.want {
			t.Fatalf("scoreToneFromRatio(%v) = %q, want %q", tt.ratio, got, tt.want)
		}
	}
}

func TestParseNumberRejectsNonFiniteValues(t *testing.T) {
	for _, value := range []string{"NaN", "+Inf", "-Inf", "Infinity"} {
		if parsed, ok := parseNumber(value); ok {
			t.Fatalf("parseNumber(%q) = %v, true; want rejected", value, parsed)
		}
	}
}
