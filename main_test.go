package main

import "testing"

func TestUsernameFromEmail(t *testing.T) {
	tests := []struct {
		email string
		want  string
	}{
		{email: "admin@valleyvista.com", want: "admin"},
		{email: "john.doe@example.com", want: "johndoe"},
		{email: "--@example.com", want: "user_--"},
		{email: "@example.com", want: "user"},
	}

	for _, tt := range tests {
		if got := usernameFromEmail(tt.email); got != tt.want {
			t.Errorf("usernameFromEmail(%q) = %q, want %q", tt.email, got, tt.want)
		}
	}
}
