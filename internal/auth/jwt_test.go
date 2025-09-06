package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWT(t *testing.T) {
	uuid1 := uuid.New()
	cases := []struct {
		inputID        uuid.UUID
		inputSecret    string
		inputExpiresIn time.Duration
		expected       uuid.UUID
	}{
		{
			inputID:        uuid1,
			inputSecret:    "secret",
			inputExpiresIn: time.Minute * 2,
			expected:       uuid1,
		},
		{
			inputID:        uuid1,
			inputSecret:    "secret",
			inputExpiresIn: time.Millisecond * 2,
			expected:       uuid.Nil,
		},
	}

	for _, c := range cases {
		tokenString, err := MakeJWT(c.inputID, c.inputSecret, c.inputExpiresIn)
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
