package meals

import (
	"context"
	"strings"
	"testing"
)

func TestForm_IncludesDirtyGuard(t *testing.T) {
	var sb strings.Builder
	if err := Form(nil, nil, "", nil).Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	out := sb.String()
	if !strings.Contains(out, "beforeunload") {
		t.Error("form should warn before discarding unsaved changes (beforeunload handler missing)")
	}
}
