package replay

import (
	"context"
	"testing"
	"time"
)

func TestMemory(t *testing.T) {
	c := NewMemory()
	until := time.Now().Add(time.Minute)
	if seen, _ := c.Seen(context.Background(), "a", until); seen {
		t.Fatal("first use is not a replay")
	}
	if seen, _ := c.Seen(context.Background(), "a", until); !seen {
		t.Fatal("second use must be a replay")
	}
}
