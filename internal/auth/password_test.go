package auth

import (
	"testing"
)

func TestHashPasswordAndCheckPasswordHash(t *testing.T) {
	cases := []struct {
		input    string
		expected error
	}{
		{
			input:    "password1",
			expected: nil,
		},
		{
			input:    "QWERTYTREWQ123*/",
			expected: nil,
		},
		{
			input:    "123456",
			expected: nil,
		},
	}

	for _, c := range cases {
		hashedPwd, _ := HashPassword(c.input)
		if err := CheckPasswordHash(c.input, hashedPwd); err != nil {
			t.Error(err)
			t.Fail()
			continue
		}
	}
}
