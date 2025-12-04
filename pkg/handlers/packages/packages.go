package packages

import (
	"context"
	"encoding/json"
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
	args, _ := request.Params.Arguments.(map[string]interface{})

	targetDir := strings.TrimSpace(packageDirectory)
	if dir, ok := getStringArgument(args, "directory"); ok {
		targetDir = dir
		if !filepath.IsAbs(targetDir) && packageDirectory != "" {
			targetDir = filepath.Join(packageDirectory, dir)
		}
	}

	if targetDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			targetDir = cwd
		}
	}

	if targetDir != "" {
		if abs, err := filepath.Abs(targetDir); err == nil {
			targetDir = abs
		}
	}

	format := "text"
	if f, ok := getStringArgument(args, "format"); ok {
		format = strings.ToLower(f)
	}

	packs, err := ListPackages(targetDir, excludeFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to scan directory: %v", err)), nil
	}

	if len(packs) == 0 {
		switch format {
		case "json":
			payload := map[string]interface{}{
				"directory": targetDir,
				"count":     0,
				"packages":  []string{},
			}
			data, marshalErr := json.MarshalIndent(payload, "", "  ")
			if marshalErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal list_packages result: %v", marshalErr)), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		default:
			return mcp.NewToolResultText(fmt.Sprintf("No DTSX files found in package directory: %s", targetDir)), nil
		}
	}

	absolute := make([]string, len(packs))
	for i, rel := range packs {
		if filepath.IsAbs(rel) {
			absolute[i] = filepath.Clean(rel)
			continue
		}
		absolute[i] = filepath.Clean(filepath.Join(targetDir, rel))
	}

	switch format {
	case "json":
		payload := map[string]interface{}{
			"directory":         targetDir,
			"count":             len(packs),
			"packages":          packs,
			"packages_absolute": absolute,
		}
		data, marshalErr := json.MarshalIndent(payload, "", "  ")
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal list_packages result: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	case "markdown":
		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("# Package Listing for %s\n\n", targetDir))
		for _, pkg := range packs {
			builder.WriteString(fmt.Sprintf("- %s\n", pkg))
		}
		return mcp.NewToolResultText(builder.String()), nil
	default:
		result := fmt.Sprintf("Found %d DTSX package(s) in directory: %s\n\n", len(packs), targetDir)
		for i, pkg := range packs {
			result += fmt.Sprintf("%d. %s\n", i+1, pkg)
		}
		return mcp.NewToolResultText(result), nil
	}
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

func getStringArgument(args map[string]interface{}, key string) (string, bool) {
	if args == nil {
		return "", false
	}
	raw, ok := args[key]
	if !ok {
		return "", false
	}
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}
