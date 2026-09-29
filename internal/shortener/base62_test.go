package shortener

import "testing"

func TestEncodeBase62(t *testing.T) {
	tests := []struct {
		id   int64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{61, "Z"},
		{62, "10"},
		{125, "21"},
	}
	for _, tt := range tests {
		if got := EncodeBase62(tt.id); got != tt.want {
			t.Errorf("EncodeBase62(%d) = %q, want %q", tt.id, got, tt.want)
		}
	}
}
