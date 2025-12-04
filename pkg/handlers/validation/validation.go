package validation

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// SSISPackage represents a minimal SSIS package for validation
type SSISPackage struct {
	Properties []Property `xml:"Property"`
}

// Property represents a property
type Property struct {
	Name  string `xml:"Name,attr"`
	Value string `xml:",innerxml"`
}

// ValidateDtsxStructure validates the basic structure of a DTSX file
func ValidateDtsxStructure(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("invalid DTSX structure: %v", err)
	}

	// Basic validation
	if len(pkg.Properties) == 0 {
		return "Validation: Warning - No properties found", nil
	}

	return "Validation: DTSX file structure is valid", nil
}

// HandleValidateDtsx handles DTSX validation requests
func HandleValidateDtsx(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := ValidateDtsxStructure(filePath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}
