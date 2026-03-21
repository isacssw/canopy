package ui

import "testing"

func TestBuildNvimRemoteOpenCommand(t *testing.T) {
	wtPath := "/tmp/my wt"
	filePath := `/tmp/my wt/a "b".go`

	got := buildNvimRemoteOpenCommand(wtPath, filePath, 42)
	want := `<C-\><C-N>:execute 'cd ' . fnameescape("/tmp/my wt") | execute 'edit +42 ' . fnameescape("/tmp/my wt/a \"b\".go")<CR>`

	if got != want {
		t.Fatalf("buildNvimRemoteOpenCommand() mismatch\n got: %q\nwant: %q", got, want)
	}
}
