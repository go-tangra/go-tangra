package observe

import "testing"

func TestSanitizeLabel(t *testing.T) {
	if got := sanitizeLabel(`a"b\c` + "\n"); got != `a\"b\\c\n` {
		t.Fatalf("got %q", got)
	}
	if promName("freya.calls") != "freya_calls" {
		t.Fatal("promName")
	}
	for _, c := range []struct {
		err  error
		want string
	}{{nil, "ok"}} {
		if classify(c.err) != c.want {
			t.Fatal("classify")
		}
	}
}
