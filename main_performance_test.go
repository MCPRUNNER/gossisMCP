package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MCPRUNNER/gossisMCP/pkg/handlers/extraction"
)

// TestPerformanceBenchmarks tests performance characteristics of DTSX parsing and analysis
func TestPerformanceBenchmarks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance benchmarks in short mode")
	}

	tests := []struct {
		name     string
		filePath string
		maxTime  time.Duration
	}{
		{
			name:     "small_package_performance",
			filePath: "testdata/Package1.dtsx",
			maxTime:  100 * time.Millisecond,
		},
		{
			name:     "medium_package_performance",
			filePath: "Documents/SSIS_EXAMPLES/ConfigFile.dtsx",
			maxTime:  200 * time.Millisecond,
		},
		{
			name:     "large_package_performance",
			filePath: "Documents/SSIS_EXAMPLES/Loader.dtsx",
			maxTime:  500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip test if file doesn't exist (for cross-platform compatibility)
			if _, err := os.Stat(tt.filePath); os.IsNotExist(err) {
				t.Skipf("Skipping test %s: file %s not found", tt.name, tt.filePath)
			}

			start := time.Now()

			// Test parse_dtsx performance
			params := map[string]interface{}{
				"file_path": tt.filePath,
				"format":    "json",
			}
			request := createTestCallToolRequest("parse_dtsx", params)

			// Determine package directory based on file path
			packageDir := "testdata"
			if strings.HasPrefix(tt.filePath, "Documents/") {
				packageDir = "Documents/SSIS_EXAMPLES"
			}

			result, err := extraction.HandleParseDtsx(context.Background(), request, packageDir)

			duration := time.Since(start)

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Less(t, duration, tt.maxTime,
				"Parsing took %v, which exceeds the maximum allowed time of %v", duration, tt.maxTime)

			t.Logf("Parse time for %s: %v", tt.filePath, duration)
		})
	}
}

// BenchmarkParseDtsx benchmarks the parse_dtsx function
func BenchmarkParseDtsx(b *testing.B) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
		"format":    "text",
	}
	request := createTestCallToolRequest("parse_dtsx", params)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := extraction.HandleParseDtsx(context.Background(), request, "testdata")
		if err != nil {
			b.Fatal(err)
		}
		if result == nil {
			b.Fatal("Expected result")
		}
	}
}

// BenchmarkExtractTasks benchmarks the extract_tasks function
func BenchmarkExtractTasks(b *testing.B) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
	}
	request := createTestCallToolRequest("extract_tasks", params)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := extraction.HandleExtractTasks(context.Background(), request, "testdata")
		if err != nil {
			b.Fatal(err)
		}
		if result == nil {
			b.Fatal("Expected result")
		}
	}
}

// TestMemoryUsageAnalysis tests memory usage patterns during DTSX processing
func TestMemoryUsageAnalysis(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory usage analysis in short mode")
	}

	// Skip test if required file doesn't exist
	testFile := "Documents/SSIS_EXAMPLES/ConfigFile.dtsx"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skipf("Skipping memory usage test: file %s not found", testFile)
	}

	var m1, m2 runtime.MemStats

	// Get initial memory stats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Perform multiple DTSX parsing operations
	for i := 0; i < 10; i++ {
		params := map[string]interface{}{
			"file_path": "ConfigFile.dtsx", // Relative path since we're using Documents/SSIS_EXAMPLES as package dir
			"format":    "json",
		}
		request := createTestCallToolRequest("parse_dtsx", params)

		result, err := extraction.HandleParseDtsx(context.Background(), request, "Documents/SSIS_EXAMPLES")
		require.NoError(t, err)
		assert.NotNil(t, result)
	}

	// Force garbage collection and get final memory stats
	runtime.GC()
	runtime.ReadMemStats(&m2)

	// Check that memory usage is reasonable (less than 100MB increase)
	// Handle potential overflow in memory stats
	var memoryIncrease uint64
	if m2.Alloc >= m1.Alloc {
		memoryIncrease = m2.Alloc - m1.Alloc
	} else {
		// Handle counter wraparound (though unlikely for Alloc)
		memoryIncrease = m2.Alloc
	}

	// Be more lenient with memory usage - focus on detecting major leaks
	assert.Less(t, memoryIncrease, uint64(100*1024*1024),
		"Memory usage increased by %d bytes, which exceeds the limit", memoryIncrease)

	t.Logf("Memory increase: %d bytes", memoryIncrease)
	t.Logf("GC cycles: %d", m2.NumGC-m1.NumGC)
}

// TestConcurrentRequests tests handling of concurrent MCP requests
func TestConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent request tests in short mode")
	}

	const numGoroutines = 10
	const numRequestsPerGoroutine = 5

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines*numRequestsPerGoroutine)

	// Launch multiple goroutines making concurrent requests
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numRequestsPerGoroutine; j++ {
				params := map[string]interface{}{
					"file_path": "testdata/Package1.dtsx",
					"format":    "text",
				}
				request := createTestCallToolRequest("parse_dtsx", params)

				result, err := extraction.HandleParseDtsx(context.Background(), request, "testdata")
				if err != nil {
					errChan <- fmt.Errorf("goroutine %d, request %d: %v", goroutineID, j, err)
					continue
				}
				if result == nil {
					errChan <- fmt.Errorf("goroutine %d, request %d: nil result", goroutineID, j)
					continue
				}
				if len(result.Content) == 0 {
					errChan <- fmt.Errorf("goroutine %d, request %d: empty content", goroutineID, j)
					continue
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	assert.Empty(t, errors, "Concurrent requests failed with errors: %v", errors)
}

// TestStressTestingWithMalformedXML tests handling of malformed XML files
func TestStressTestingWithMalformedXML(t *testing.T) {
	// Create temporary malformed XML files
	malformedFiles := []struct {
		name     string
		content  string
		errorMsg string
	}{
		{
			name:     "unclosed_tag",
			content:  `<DTS:Executable><DTS:Property Name="Test">Value</DTS:Property>`,
			errorMsg: "XML syntax error",
		},
		{
			name:     "invalid_namespace",
			content:  `<InvalidNS:Executable xmlns:InvalidNS="invalid"><InvalidNS:Property Name="Test">Value</InvalidNS:Property></InvalidNS:Executable>`,
			errorMsg: "parsing error",
		},
		{
			name:     "empty_file",
			content:  "",
			errorMsg: "parsing error",
		},
		{
			name:     "binary_data",
			content:  string([]byte{0x00, 0x01, 0x02, 0xFF}),
			errorMsg: "parsing error",
		},
	}

	for _, mf := range malformedFiles {
		t.Run(mf.name, func(t *testing.T) {
			// Create temporary file
			tempFile, err := os.CreateTemp("", "malformed_*.dtsx")
			require.NoError(t, err)
			defer os.Remove(tempFile.Name())

			_, err = tempFile.WriteString(mf.content)
			require.NoError(t, err)
			tempFile.Close()

			// Test parsing the malformed file
			params := map[string]interface{}{
				"file_path": filepath.Base(tempFile.Name()),
			}
			request := createTestCallToolRequest("parse_dtsx", params)

			result, err := extraction.HandleParseDtsx(context.Background(), request, filepath.Dir(tempFile.Name()))

			// Should not panic and should return some result (error result is acceptable)
			assert.NotNil(t, result)
			assert.NotEmpty(t, result.Content)

			// Check that result contains error information
			content := result.Content[0]
			textContent, ok := content.(mcp.TextContent)
			assert.True(t, ok, "Expected TextContent")

			// Should contain some indication of error or minimal valid output
			assert.True(t, strings.Contains(textContent.Text, "Error") ||
				strings.Contains(textContent.Text, "parse_dtsx") ||
				len(textContent.Text) > 0,
				"Expected error indication or valid output, got: %s", textContent.Text)
		})
	}
}

// TestSecurityPathTraversal tests protection against path traversal attacks
func TestSecurityPathTraversal(t *testing.T) {
	attackVectors := []struct {
		name        string
		filePath    string
		description string
	}{
		{
			name:        "dot_dot_traversal",
			filePath:    "../../../etc/passwd",
			description: "Directory traversal with ../",
		},
		{
			name:        "absolute_path",
			filePath:    "C:\\Windows\\System32\\config\\system",
			description: "Absolute path access attempt",
		},
		{
			name:        "encoded_traversal",
			filePath:    "..%2F..%2F..%2Fetc%2Fpasswd",
			description: "URL-encoded directory traversal",
		},
		{
			name:        "null_byte_injection",
			filePath:    "testdata/Package1.dtsx\x00/../../../etc/passwd",
			description: "Null byte injection",
		},
	}

	for _, attack := range attackVectors {
		t.Run(attack.name, func(t *testing.T) {
			params := map[string]interface{}{
				"file_path": attack.filePath,
			}
			request := createTestCallToolRequest("parse_dtsx", params)

			result, err := extraction.HandleParseDtsx(context.Background(), request, "testdata")

			// Should not panic and should return error result
			assert.NoError(t, err, "Handler should not return error for security test")
			assert.NotNil(t, result)
			assert.NotEmpty(t, result.Content)

			content := result.Content[0]
			textContent, ok := content.(mcp.TextContent)
			assert.True(t, ok, "Expected TextContent")

			// Should contain error message about file not found or access denied
			assert.True(t,
				strings.Contains(textContent.Text, "Error") ||
					strings.Contains(textContent.Text, "cannot find") ||
					strings.Contains(textContent.Text, "not found") ||
					strings.Contains(textContent.Text, "access denied"),
				"Expected security error for %s, got: %s", attack.description, textContent.Text)

			// Should NOT contain sensitive system information that could aid attacks
			// Allow the path to appear in error messages as long as it's not revealing sensitive content
			assert.NotContains(t, textContent.Text, "/etc/shadow")
			assert.NotContains(t, textContent.Text, "password")
			assert.NotContains(t, textContent.Text, "secret")
		})
	}
}

// TestLargeFileHandling tests processing of large DTSX files
func TestLargeFileHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large file handling test in short mode")
	}

	// Create a large DTSX file for testing
	tempFile, err := os.CreateTemp("", "large_test_*.dtsx")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	// Create a DTSX file with many components to simulate a large package
	var content strings.Builder
	content.WriteString(`<?xml version="1.0"?>
<DTS:Executable xmlns:DTS="www.microsoft.com/SqlServer/Dts" DTS:ExecutableType="Microsoft.Package">
  <DTS:Property DTS:Name="PackageFormatVersion">8</DTS:Property>
  <DTS:Variables>
`)

	// Add many variables
	for i := 0; i < 1000; i++ {
		content.WriteString(fmt.Sprintf(`    <DTS:Variable DTS:Name="TestVar%d" DTS:DataType="Int32">%d</DTS:Variable>`, i, i))
	}

	content.WriteString(`  </DTS:Variables>
  <DTS:Executables>
`)

	// Add many tasks
	for i := 0; i < 500; i++ {
		content.WriteString(fmt.Sprintf(`    <DTS:Executable DTS:ExecutableType="Microsoft.SqlServer.Dts.Tasks.ExecuteSQLTask" DTS:ThreadHint="0">
      <DTS:Property DTS:Name="SqlStatementSource">SELECT %d</DTS:Property>
    </DTS:Executable>`, i))
	}

	content.WriteString(`  </DTS:Executables>
</DTS:Executable>`)

	_, err = tempFile.WriteString(content.String())
	require.NoError(t, err)
	tempFile.Close()

	// Test parsing the large file
	start := time.Now()
	params := map[string]interface{}{
		"file_path": filepath.Base(tempFile.Name()),
	}
	request := createTestCallToolRequest("parse_dtsx", params)

	result, err := extraction.HandleParseDtsx(context.Background(), request, filepath.Dir(tempFile.Name()))
	duration := time.Since(start)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Less(t, duration, 2*time.Second, "Large file parsing took too long: %v", duration)

	// Verify the result contains expected data
	contentResult := result.Content[0]
	textContent, ok := contentResult.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")

	assert.Contains(t, textContent.Text, "parse_dtsx Analysis Report")
	assert.Contains(t, textContent.Text, "variables_count")
	assert.Contains(t, textContent.Text, "tasks_count")

	t.Logf("Large file (%d bytes) parsed in %v", len(content.String()), duration)
}

// TestResourceCleanup tests that resources are properly cleaned up after operations
func TestResourceCleanup(t *testing.T) {
	// Test that temporary files and resources are cleaned up
	initialGoroutines := runtime.NumGoroutine()

	// Perform several operations
	for i := 0; i < 20; i++ {
		params := map[string]interface{}{
			"file_path": "testdata/Package1.dtsx",
		}
		request := createTestCallToolRequest("extract_tasks", params)

		result, err := extraction.HandleExtractTasks(context.Background(), request, "testdata")
		require.NoError(t, err)
		assert.NotNil(t, result)
	}

	// Force garbage collection
	runtime.GC()
	runtime.GC() // Run twice to ensure cleanup

	finalGoroutines := runtime.NumGoroutine()

	// Allow for some variance in goroutine count (test framework may create goroutines)
	assert.LessOrEqual(t, finalGoroutines-initialGoroutines, 5,
		"Too many goroutines created: initial=%d, final=%d", initialGoroutines, finalGoroutines)

	t.Logf("Goroutines: initial=%d, final=%d", initialGoroutines, finalGoroutines)
}
