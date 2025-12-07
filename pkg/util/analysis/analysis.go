package analysis

import (
	"fmt"
	"regexp"
	"strings"
)

// AnalyzeBatchFile analyzes batch file content and extracts variables, commands, and calls
func AnalyzeBatchFile(content string, isLineNumberNeeded bool, result *strings.Builder) {
	lines := strings.Split(content, "\n")
	//var lines []string
	//lines = convertToLines(content,isLineNumberNeeded)

	var variables []string
	var commands []string
	var calls []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "REM") || strings.HasPrefix(line, "::") {
			continue
		}

		// Extract variables (SET commands)
		if strings.HasPrefix(strings.ToUpper(line), "SET ") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				varName := strings.TrimPrefix(strings.ToUpper(parts[0]), "SET ")
				varName = strings.TrimSpace(varName)
				if varName != "" {
					variables = append(variables, varName)
				}
			}
		}

		// Extract commands
		if strings.Contains(line, ".exe") || strings.Contains(line, ".bat") || strings.Contains(line, ".cmd") {
			commands = append(commands, line)
		}

		// Extract CALL statements
		if strings.HasPrefix(strings.ToUpper(line), "CALL ") {
			call := strings.TrimPrefix(line, "CALL ")
			call = strings.TrimSpace(call)
			if call != "" {
				calls = append(calls, call)
			}
		}
	}

	result.WriteString("Batch File Analysis:\n")
	result.WriteString(fmt.Sprintf("- Variables found: %d\n", len(variables)))
	if len(variables) > 0 {
		result.WriteString("  Variables:\n")
		for _, v := range variables {
			result.WriteString(fmt.Sprintf("    - %s\n", v))
		}
	}

	result.WriteString(fmt.Sprintf("- Commands found: %d\n", len(commands)))
	if len(commands) > 0 {
		result.WriteString("  Commands:\n")
		for _, c := range commands {
			result.WriteString(fmt.Sprintf("    - %s\n", c))
		}
	}

	result.WriteString(fmt.Sprintf("- CALL statements: %d\n", len(calls)))
	if len(calls) > 0 {
		result.WriteString("  Calls:\n")
		for _, c := range calls {
			result.WriteString(fmt.Sprintf("    - %s\n", c))
		}
	}
}

// AnalyzeConfigFile analyzes configuration file content
func AnalyzeConfigFile(content string, isLineNumberNeeded bool, result *strings.Builder) {
	lines := strings.Split(content, "\n")

	result.WriteString("Configuration File Analysis:\n")

	sections := make(map[string][]string)
	var currentSection string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Check for section headers [section]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line[1 : len(line)-1]
			continue
		}

		// Parse key=value pairs
		if strings.Contains(line, "=") && currentSection != "" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				entry := fmt.Sprintf("%s=%s", key, value)
				sections[currentSection] = append(sections[currentSection], entry)
			}
		}
	}

	result.WriteString(fmt.Sprintf("- Sections found: %d\n", len(sections)))
	for section, entries := range sections {
		result.WriteString(fmt.Sprintf("  Section [%s]: %d entries\n", section, len(entries)))
		for _, entry := range entries {
			result.WriteString(fmt.Sprintf("    - %s\n", entry))
		}
	}
}

// AnalyzeSQLFile analyzes SQL file content
func AnalyzeSQLFile(content string, isLineNumberNeeded bool, result *strings.Builder) {

	result.WriteString("SQL File Analysis:\n")

	selectCount := strings.Count(strings.ToUpper(content), "SELECT ")
	insertCount := strings.Count(strings.ToUpper(content), "INSERT ")
	updateCount := strings.Count(strings.ToUpper(content), "UPDATE ")
	deleteCount := strings.Count(strings.ToUpper(content), "DELETE ")
	createCount := strings.Count(strings.ToUpper(content), "CREATE ")
	dropCount := strings.Count(strings.ToUpper(content), "DROP ")

	result.WriteString(fmt.Sprintf("- SELECT statements: %d\n", selectCount))
	result.WriteString(fmt.Sprintf("- INSERT statements: %d\n", insertCount))
	result.WriteString(fmt.Sprintf("- UPDATE statements: %d\n", updateCount))
	result.WriteString(fmt.Sprintf("- DELETE statements: %d\n", deleteCount))
	result.WriteString(fmt.Sprintf("- CREATE statements: %d\n", createCount))
	result.WriteString(fmt.Sprintf("- DROP statements: %d\n", dropCount))

	// Check for potential SSIS-related patterns
	ssisPatterns := []string{
		"EXECUTE", "BULK INSERT", "OPENROWSET", "OPENDATASOURCE",
		"sp_executesql", "xp_cmdshell",
	}

	foundPatterns := make(map[string]int)
	for _, pattern := range ssisPatterns {
		count := strings.Count(strings.ToUpper(content), strings.ToUpper(pattern))
		if count > 0 {
			foundPatterns[pattern] = count
		}
	}

	if len(foundPatterns) > 0 {
		result.WriteString("- SSIS-related patterns found:\n")
		for pattern, count := range foundPatterns {
			result.WriteString(fmt.Sprintf("  - %s: %d\n", pattern, count))
		}
	}
}

// AnalyzeGenericTextFile provides basic analysis of generic text files
func AnalyzeGenericTextFile(content string, isLineNumberNeeded bool, result *strings.Builder) {
	lines := strings.Split(content, "\n")

	result.WriteString("Generic Text File Analysis:\n")
	result.WriteString(fmt.Sprintf("- Total lines: %d\n", len(lines)))

	// Count non-empty lines
	nonEmptyLines := 0
	totalChars := 0

	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
		totalChars += len(line)
	}

	result.WriteString(fmt.Sprintf("- Non-empty lines: %d\n", nonEmptyLines))
	result.WriteString(fmt.Sprintf("- Total characters: %d\n", totalChars))
	result.WriteString(fmt.Sprintf("- Average line length: %.1f characters\n", float64(totalChars)/float64(len(lines))))

	// Check for common patterns
	if strings.Contains(content, "<?xml") {
		result.WriteString("- Appears to be XML content\n")
	} else if strings.Contains(content, "{") && strings.Contains(content, "}") {
		result.WriteString("- Appears to contain JSON-like structures\n")
	} else if regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`).MatchString(content) {
		result.WriteString("- Contains IP addresses\n")
	}
}
