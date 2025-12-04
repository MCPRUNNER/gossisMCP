package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"
)

// RunnerFunc describes a function capable of invoking an MCP tool.
// The function receives the tool name and fully resolved parameters, and it
// returns the raw string payload produced by the tool. The caller is
// responsible for converting rich results into a string when necessary.
type RunnerFunc func(ctx context.Context, tool string, params map[string]interface{}) (string, error)

// Workflow represents an ordered collection of workflow steps.
type Workflow struct {
	Steps []Step `json:"Steps" yaml:"Steps"`
}

// Step models a single workflow operation.
type Step struct {
	Name       string                 `json:"Name" yaml:"Name"`
	Type       string                 `json:"Type" yaml:"Type"`
	Parameters map[string]interface{} `json:"Parameters" yaml:"Parameters"`
	Enabled    bool                   `json:"Enabled" yaml:"Enabled"`
	Output     *StepOutput            `json:"Output" yaml:"Output"`
}

// StepOutput declares the named output captured from a workflow step.
type StepOutput struct {
	Name   string `json:"Name" yaml:"Name"`
	Format string `json:"Format" yaml:"Format"`
}

// StepResult captures the resolved value emitted by a workflow step.
type StepResult struct {
	Value  string
	Format string
}

var placeholderExpr = regexp.MustCompile(`\{([A-Za-z0-9_-]+)\.([A-Za-z0-9_-]+)\}`)

// LoadFromFile loads a workflow definition from JSON or YAML.
func LoadFromFile(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	wf := &Workflow{}
	if err := json.Unmarshal(data, wf); err != nil {
		if yamlErr := yaml.Unmarshal(data, wf); yamlErr != nil {
			return nil, fmt.Errorf("failed to parse workflow %s: %v, %v", path, err, yamlErr)
		}
	}

	if err := wf.Validate(); err != nil {
		return nil, err
	}

	return wf, nil
}

// Validate ensures the workflow structure is runnable.
func (wf *Workflow) Validate() error {
	if wf == nil {
		return errors.New("workflow is nil")
	}
	if len(wf.Steps) == 0 {
		return errors.New("workflow contains no steps")
	}

	seenNames := make(map[string]struct{})
	for i, step := range wf.Steps {
		if strings.TrimSpace(step.Name) == "" {
			return fmt.Errorf("step %d is missing a Name", i)
		}
		if _, exists := seenNames[step.Name]; exists {
			return fmt.Errorf("duplicate step name detected: %s", step.Name)
		}
		seenNames[step.Name] = struct{}{}

		if strings.TrimSpace(step.Type) == "" {
			return fmt.Errorf("step %s is missing a Type", step.Name)
		}
	}
	return nil
}

// Execute walks each enabled step, resolving parameter placeholders and invoking the provided runner.
func (wf *Workflow) Execute(ctx context.Context, runner RunnerFunc) (map[string]map[string]StepResult, error) {
	if runner == nil {
		return nil, errors.New("runner cannot be nil")
	}
	if err := wf.Validate(); err != nil {
		return nil, err
	}

	results := make(map[string]map[string]StepResult)

	for _, step := range wf.Steps {
		if !step.Enabled {
			continue
		}

		resolvedParams := make(map[string]interface{}, len(step.Parameters))
		for key, value := range step.Parameters {
			resolved, err := resolveParameterValue(value, results)
			if err != nil {
				return nil, fmt.Errorf("step %s parameter %s: %w", step.Name, key, err)
			}
			resolvedParams[key] = resolved
		}

		toolName := strings.TrimPrefix(step.Type, "#")
		outputValue, err := runner(ctx, toolName, resolvedParams)
		if err != nil {
			return nil, fmt.Errorf("step %s: %w", step.Name, err)
		}

		if results[step.Name] == nil {
			results[step.Name] = make(map[string]StepResult)
		}

		if step.Output != nil && step.Output.Name != "" {
			results[step.Name][step.Output.Name] = StepResult{Value: outputValue, Format: step.Output.Format}
		} else {
			// Default output name when none is specified
			results[step.Name]["Result"] = StepResult{Value: outputValue}
		}
	}

	return results, nil
}

func resolveParameterValue(value interface{}, outputs map[string]map[string]StepResult) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return resolvePlaceholderString(v, outputs)
	case []interface{}:
		resolved := make([]interface{}, len(v))
		for i, item := range v {
			out, err := resolveParameterValue(item, outputs)
			if err != nil {
				return nil, err
			}
			resolved[i] = out
		}
		return resolved, nil
	case map[string]interface{}:
		resolved := make(map[string]interface{}, len(v))
		for key, item := range v {
			out, err := resolveParameterValue(item, outputs)
			if err != nil {
				return nil, err
			}
			resolved[key] = out
		}
		return resolved, nil
	default:
		return value, nil
	}
}

func resolvePlaceholderString(input string, outputs map[string]map[string]StepResult) (string, error) {
	matches := placeholderExpr.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return input, nil
	}

	result := input
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}
		stepName, outputName := match[1], match[2]
		stepOutputs, ok := outputs[stepName]
		if !ok {
			return "", fmt.Errorf("referenced step %q has not produced outputs", stepName)
		}
		output, ok := stepOutputs[outputName]
		if !ok {
			return "", fmt.Errorf("step %q does not contain output %q", stepName, outputName)
		}
		result = strings.ReplaceAll(result, match[0], output.Value)
	}

	return result, nil
}

// ResolveRelativePath expands paths relative to the workflow file location.
func ResolveRelativePath(workflowPath, target string) string {
	if workflowPath == "" || filepath.IsAbs(target) {
		return target
	}
	base := filepath.Dir(workflowPath)
	return filepath.Join(base, target)
}

// ToolResultToString extracts a textual representation from an MCP tool result.
func ToolResultToString(result *mcp.CallToolResult) (string, error) {
	if result == nil {
		return "", errors.New("tool result is nil")
	}

	var parts []string
	for _, content := range result.Content {
		if textContent, ok := mcp.AsTextContent(content); ok {
			parts = append(parts, textContent.Text)
		}
	}

	if len(parts) == 0 && result.StructuredContent != nil {
		data, err := json.MarshalIndent(result.StructuredContent, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to encode structured tool result: %w", err)
		}
		parts = append(parts, string(data))
	}

	combined := strings.TrimSpace(strings.Join(parts, "\n"))
	if result.IsError {
		if combined == "" {
			combined = "tool execution failed"
		}
		return "", errors.New(combined)
	}

	if combined == "" {
		return "", errors.New("tool result did not contain textual content")
	}

	return combined, nil
}
