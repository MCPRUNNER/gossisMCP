package packages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// ListPackages scans a directory for DTSX files
func ListPackages(packageDirectory string) ([]string, error) {
	var packages []string

	// Walk the package directory recursively to find .dtsx files
	err := filepath.Walk(packageDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".dtsx" {
			// Get relative path from package directory
			relPath, err := filepath.Rel(packageDirectory, path)
			if err != nil {
				relPath = path // fallback to absolute path if relative fails
			}
			packages = append(packages, relPath)
		}
		return nil
	})

	return packages, err
}

// HandleListPackages handles package listing requests
func HandleListPackages(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	packages, err := ListPackages(packageDirectory)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to scan directory: %v", err)), nil
	}

	if len(packages) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No DTSX files found in package directory: %s", packageDirectory)), nil
	}

	result := fmt.Sprintf("Found %d DTSX package(s) in directory: %s\n\n", len(packages), packageDirectory)
	for i, pkg := range packages {
		result += fmt.Sprintf("%d. %s\n", i+1, pkg)
	}

	return mcp.NewToolResultText(result), nil
}
