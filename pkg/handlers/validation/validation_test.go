package validation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateDtsxStructure(t *testing.T) {
	validPath := filepath.Join("..", "..", "..", "testdata", "ConfigFile.dtsx")
	if _, err := ValidateDtsxStructure(validPath); err != nil {
		t.Fatalf("expected valid structure, got error %v", err)
	}

	tempFile := filepath.Join(t.TempDir(), "invalid.dtsx")
	if err := os.WriteFile(tempFile, []byte("<not xml>"), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if _, err := ValidateDtsxStructure(tempFile); err == nil {
		t.Fatal("expected invalid xml to return error")
	}
}
