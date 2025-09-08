package auth

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestJWT(t *testing.T) {
	uuid1 := uuid.New()
	cases := []struct {
		inputID     uuid.UUID
		inputSecret string
		expected    uuid.UUID
	}{
		{
			inputID:     uuid1,
			inputSecret: "secret",
			expected:    uuid1,
		},
	}

	for _, c := range cases {
		tokenString, err := MakeJWT(c.inputID, c.inputSecret)
		if err != nil {
			t.Errorf("failed to create JWT: %v\n", err)
			t.Fail()
			continue
		}
		actual, err := ValidateJWT(tokenString, c.inputSecret)
		if actual != c.expected {
			t.Errorf("error: %v\nexpected: %v\nreceived: %v", err, c.expected, actual)
			t.Fail()
		}
	}
}

func TestGetBearerToken(t *testing.T) {
	headers1 := http.Header{}
	headers1.Set("Authorization", "Bearer 123456")
	headers2 := http.Header{}
	cases := []struct {
		input    http.Header
		expected string
	}{
		{
			input:    headers1,
			expected: "123456",
		},
		{
			input:    headers2,
			expected: "",
		},
	}

	for _, c := range cases {
		actual, _ := GetBearerToken(c.input)
		if actual != c.expected {
			t.Errorf("expected: %v\nreceived: %v", c.expected, actual)
			t.Fail()
		}

	}
}
