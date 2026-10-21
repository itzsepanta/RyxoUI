package diff

import "testing"

func TestTheDifferenceIsFound(t *testing.T) {
	before := "{\n  \"a\": 1\n}"
	after := "{\n  \"a\": 2\n}"

	lines := Lines(before, after)
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want 4", len(lines))
	}
	if lines[0].Kind != Same {
		t.Fatalf("the first line is %v", lines[0])
	}
	if lines[1].Kind != Removed || lines[1].Text != "  \"a\": 1" {
		t.Fatalf("the second line is %v", lines[1])
	}
	if lines[2].Kind != Added || lines[2].Text != "  \"a\": 2" {
		t.Fatalf("the third line is %v", lines[2])
	}
	if !Changed(before, after) {
		t.Fatal("the texts differ")
	}
	if Changed(before, before) {
		t.Fatal("the texts are the same")
	}
}

func TestTheSameTextHasNoDifference(t *testing.T) {
	text := "{\n  \"a\": 1\n}"
	lines := Lines(text, text)
	for _, line := range lines {
		if line.Kind != Same {
			t.Fatalf("the line %v is not the same", line)
		}
	}
}

func TestANewTextIsAllAdded(t *testing.T) {
	lines := Lines("", "one\ntwo")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	for _, line := range lines {
		if line.Kind != Added {
			t.Fatalf("the line %v is not added", line)
		}
	}
}
