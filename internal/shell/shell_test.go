package shell

import "testing"

func TestShQuote(t *testing.T) {
	if got := Quote("it's"); got != `'it'\''s'` {
		t.Errorf("got %s", got)
	}
}
