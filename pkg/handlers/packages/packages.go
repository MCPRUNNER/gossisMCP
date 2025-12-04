package packages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

const excludeFileName = ".gossisignore"

// ListPackages scans a directory for DTSX files
func ListPackages(packageDirectory, excludeFile string) ([]string, error) {
	var packages []string

	excludePatterns, err := loadExcludePatterns(packageDirectory, excludeFile)
	if err != nil {
		return nil, err
	}

	// Walk the package directory recursively to find .dtsx files
	err = filepath.Walk(packageDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, relErr := filepath.Rel(packageDirectory, path)
		if relErr != nil {
			relPath = path
		}

		if shouldExcludePath(relPath, info.IsDir(), excludePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".dtsx" {
			// Get relative path from package directory
			if relErr != nil {
				relPath = path // fallback to absolute path if relative fails
			}
			packages = append(packages, relPath)
		}
		return nil
	})

	return packages, err
}

func HandleListPackages(_ context.Context, request mcp.CallToolRequest, packageDirectory, excludeFile string) (*mcp.CallToolResult, error) {
	packs, err := ListPackages(packageDirectory, excludeFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to scan directory: %v", err)), nil
	}

	if len(packs) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No DTSX files found in package directory: %s", packageDirectory)), nil
	}

	result := fmt.Sprintf("Found %d DTSX package(s) in directory: %s\n\n", len(packs), packageDirectory)
	for i, pkg := range packs {
		result += fmt.Sprintf("%d. %s\n", i+1, pkg)
	}

	return mcp.NewToolResultText(result), nil
}

func loadExcludePatterns(packageDirectory, excludeFile string) ([]string, error) {
	if packageDirectory == "" {
		packageDirectory = "."
	}

	path := strings.TrimSpace(excludeFile)
	if path == "" {
		path = filepath.Join(packageDirectory, excludeFileName)
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(packageDirectory, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read exclude file %s: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	patterns := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		patterns = append(patterns, trimmed)
	}
	return patterns, nil
}

func shouldExcludePath(relPath string, isDir bool, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	rel := filepath.ToSlash(relPath)
	if rel == "." {
		rel = ""
	}
	rel = strings.TrimPrefix(rel, "./")

	for _, raw := range patterns {
		pattern, ok := normalizePattern(raw)
		if !ok {
			continue
		}

		if strings.HasSuffix(pattern, "/") {
			base := strings.TrimSuffix(pattern, "/")
			if base == "" {
				continue
			}
			if rel == base || strings.HasPrefix(rel, base+"/") {
				return true
			}
			continue
		}

		if !hasGlobMeta(pattern) {
			if rel == pattern || strings.HasPrefix(rel, pattern+"/") {
				return true
			}
			continue
		}

		if match, _ := path.Match(pattern, rel); match {
			return true
		}
	}

	return false
}

func normalizePattern(pattern string) (string, bool) {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	trimmed = strings.TrimPrefix(trimmed, "./")
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	return trimmed, true
}

func hasGlobMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}
