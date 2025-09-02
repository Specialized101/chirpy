package main

import (
	"testing"
)

func TestCensorBadWords(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			input:    "I had something interesting for breakfast",
			expected: "I had something interesting for breakfast",
		},
		{
			input:    "I hear Mastodon is better than Chirpy. sharbert I need to migrate",
			expected: "I hear Mastodon is better than Chirpy. **** I need to migrate",
		},
		{
			input:    "I really need a kerfuffle to go to bed sooner, Fornax !",
			expected: "I really need a **** to go to bed sooner, **** !",
		},
		{
			input:    "I really need a KERFUFFLE to go to bed sooner, Fornax !",
			expected: "I really need a **** to go to bed sooner, **** !",
		},
		{
			input:    "I really need a kerfuffle to go to bed sooner, Fornax!",
			expected: "I really need a **** to go to bed sooner, Fornax!",
		},
	}

	for _, c := range cases {
		actual := censorBadWords(c.input)
		if actual != c.expected {
			t.Errorf("wrong output, expected %v but received %v", c.expected, actual)
			t.Fail()
		}
	}
}
