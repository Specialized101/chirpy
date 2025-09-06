package auth

import (
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
		}
		actual, err := ValidateJWT(tokenString, c.inputSecret)
		if actual != c.expected {
			t.Errorf("expected : %v\n", err)
			t.Fail()
		}
	}
}
