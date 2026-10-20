// Package enginetest holds the checks every engine has to pass.
//
// An adapter that follows the contract has to behave the same way, whatever
// the protocol is. These checks are that behaviour, written once.
package enginetest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ryxoui/ryxoui/internal/engines"
)

// Conformance runs the checks of the contract against an engine.
func Conformance(t *testing.T, engine engines.Engine, config engines.Config) {
	t.Helper()

	t.Run("the engine has a name", func(t *testing.T) {
		if engine.Name() == "" {
			t.Fatal("the engine must have a name")
		}
	})

	t.Run("the engine is in the catalogue", func(t *testing.T) {
		found, err := engines.Get(engine.Name())
		if err != nil {
			t.Fatalf("the engine %q is not in the catalogue: %v", engine.Name(), err)
		}
		if found.Name() != engine.Name() {
			t.Fatalf("the catalogue holds %q, want %q", found.Name(), engine.Name())
		}
	})

	t.Run("the rendering is a document", func(t *testing.T) {
		content, err := engine.Render(context.Background(), engines.Target{}, config)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if len(content) == 0 {
			t.Fatal("the rendering must not be empty")
		}
		var document map[string]any
		if err := json.Unmarshal(content, &document); err != nil {
			t.Fatalf("the rendering is not a document: %v", err)
		}
		if len(document) == 0 {
			t.Fatal("the rendering must hold something")
		}
	})

	t.Run("the rendering is stable", func(t *testing.T) {
		first, err := engine.Render(context.Background(), engines.Target{}, config)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		second, err := engine.Render(context.Background(), engines.Target{}, config)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if string(first) != string(second) {
			t.Fatal("the same configuration must render the same way")
		}
	})

	t.Run("the users reach the configuration", func(t *testing.T) {
		withUser := config
		withUser.Users = []engines.User{
			{ID: "u1", UUID: "11111111-1111-1111-1111-111111111111", Email: "one@example", Enabled: true},
		}

		content, err := engine.Render(context.Background(), engines.Target{}, withUser)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if !contains(content, "11111111-1111-1111-1111-111111111111") {
			t.Fatalf("the user is missing in the rendering: %s", content)
		}
	})
}

// contains reports whether the text holds the piece.
func contains(text []byte, piece string) bool {
	return len(text) > 0 && len(piece) > 0 && indexOf(string(text), piece) >= 0
}

// indexOf finds a piece in a text.
func indexOf(text, piece string) int {
	for i := 0; i+len(piece) <= len(text); i++ {
		if text[i:i+len(piece)] == piece {
			return i
		}
	}
	return -1
}
