package gmailotp

import (
	"testing"

	"dnse-mt5-connector/internal/config"
)

func TestParseOTP(t *testing.T) {
	s := NewService(config.GmailOTPConfig{}, nil)
	// Override regex to ensure it's initialized (handled in NewService)

	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "plain text OTP",
			body:     "Your OTP is 123456. Do not share.",
			expected: "123456",
		},
		{
			name:     "HTML OTP",
			body:     "<div>Mã OTP: <b>654321</b></div>",
			expected: "654321",
		},
		{
			name:     "multiple numbers",
			body:     "Contact 19001234. Your code is 112233.",
			expected: "112233",
		},
		{
			name:     "no OTP",
			body:     "Your code is 12345 (too short).",
			expected: "",
		},
		{
			name:     "7 digit number",
			body:     "Your code is 1234567 (too long).",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.parseOTP(tt.body)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
