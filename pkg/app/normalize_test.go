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
