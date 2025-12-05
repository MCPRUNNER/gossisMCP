package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	Name           string                 `json:"Name" yaml:"Name"`
	Type           string                 `json:"Type" yaml:"Type"`
	Parameters     map[string]interface{} `json:"Parameters" yaml:"Parameters"`
	Enabled        bool                   `json:"Enabled" yaml:"Enabled"`
	Output         *StepOutput            `json:"Output" yaml:"Output"`
	Loop           *LoopConfig            `json:"loop" yaml:"loop"`
	OutputFilePath string                 `json:"output_file_path" yaml:"output_file_path"`
}

// StepOutput declares the named output captured from a workflow step.
type StepOutput struct {
	Name   string `json:"Name" yaml:"Name"`
	Format string `json:"Format" yaml:"Format"`
}

// LoopConfig describes a fan-out configuration for a step.
// input_data should resolve to a JSON array or an object containing an array field.
// Each element is substituted into parameters via {item_name}.
type LoopConfig struct {
	InputData string `json:"input_data" yaml:"input_data"`
	ItemName  string `json:"item_name" yaml:"item_name"`
}

// StepResult captures the resolved value emitted by a workflow step.
type StepResult struct {
	Value  string
	Format string
}

var placeholderExpr = regexp.MustCompile(`\{([A-Za-z0-9_-]+)\.([A-Za-z0-9_.-]+)\}`)

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

		if step.Loop != nil {
			if strings.TrimSpace(step.Loop.InputData) == "" {
				return fmt.Errorf("step %s loop is missing input_data", step.Name)
			}
			if strings.TrimSpace(step.Loop.ItemName) == "" {
				return fmt.Errorf("step %s loop is missing item_name", step.Name)
			}
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

		if step.Loop != nil {
			loopItems, err := resolveLoopItems(step.Loop, results)
			if err != nil {
				return nil, fmt.Errorf("step %s loop: %w", step.Name, err)
			}

			var aggregated []string
			for idx, item := range loopItems {
				resolvedParams := make(map[string]interface{}, len(step.Parameters))
				for key, value := range step.Parameters {
					resolved, err := resolveParameterValue(value, results)
					if err != nil {
						return nil, fmt.Errorf("step %s parameter %s (loop %d): %w", step.Name, key, idx, err)
					}
					// If this is an output file path, expand the placeholder into a safe filename
					if strVal, ok := resolved.(string); ok && key == "output_file_path" {
						if strings.Contains(strVal, fmt.Sprintf("{%s}", step.Loop.ItemName)) {
							// Derive a safe filename from the item (strip dirs and extension)
							base := filepath.Base(item)
							name := strings.TrimSuffix(base, filepath.Ext(base))
							resolvedParams[key] = strings.ReplaceAll(strVal, fmt.Sprintf("{%s}", step.Loop.ItemName), name)
							continue
						}
					}
					resolvedParams[key] = applyLoopItem(resolved, step.Loop.ItemName, item)
				}
				if step.OutputFilePath != "" {
					if _, exists := resolvedParams["output_file_path"]; !exists {
						resolvedParams["output_file_path"] = step.OutputFilePath
					}
				}

				toolName := strings.TrimPrefix(step.Type, "#")
				outputValue, err := runner(ctx, toolName, resolvedParams)
				if err != nil {
					return nil, fmt.Errorf("step %s (loop %d): %w", step.Name, idx, err)
				}
				aggregated = append(aggregated, outputValue)
			}

			if results[step.Name] == nil {
				results[step.Name] = make(map[string]StepResult)
			}

			joined := strings.Join(aggregated, "\n")
			if step.Output != nil && step.Output.Name != "" {
				results[step.Name][step.Output.Name] = StepResult{Value: joined, Format: step.Output.Format}
			} else {
				results[step.Name]["Result"] = StepResult{Value: joined}
			}

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
		if step.OutputFilePath != "" {
			resolvedParams["output_file_path"] = step.OutputFilePath
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
		stepName, outputPath := match[1], match[2]
		stepOutputs, ok := outputs[stepName]
		if !ok {
			return "", fmt.Errorf("referenced step %q has not produced outputs", stepName)
		}

		parts := strings.SplitN(outputPath, ".", 2)
		baseOutput := parts[0]
		output, ok := stepOutputs[baseOutput]
		if !ok {
			return "", fmt.Errorf("step %q does not contain output %q", stepName, baseOutput)
		}

		replacement := output.Value
		if len(parts) == 2 {
			extracted, err := extractFieldFromJSON(output.Value, parts[1])
			if err != nil {
				return "", fmt.Errorf("step %q output %q: %w", stepName, outputPath, err)
			}
			replacement = extracted
		}

		result = strings.ReplaceAll(result, match[0], replacement)
	}

	return result, nil
}

func resolveLoopItems(loop *LoopConfig, outputs map[string]map[string]StepResult) ([]string, error) {
	if loop == nil {
		return nil, nil
	}

	resolvedInput, err := resolvePlaceholderString(loop.InputData, outputs)
	if err != nil {
		return nil, err
	}

	var asAny interface{}
	if json.Unmarshal([]byte(resolvedInput), &asAny) == nil {
		switch v := asAny.(type) {
		case []interface{}:
			return stringifySlice(v), nil
		case map[string]interface{}:
			for _, key := range []string{"packages_absolute", "packages", "items", "files"} {
				if arr, ok := v[key].([]interface{}); ok {
					return stringifySlice(arr), nil
				}
			}
			return nil, fmt.Errorf("loop input_data JSON object did not contain an array field")
		case string:
			return []string{v}, nil
		default:
			return nil, fmt.Errorf("loop input_data JSON is not an array")
		}
	}

	fields := splitToFields(resolvedInput)
	if len(fields) == 0 {
		return nil, fmt.Errorf("loop input_data did not yield any items")
	}
	return fields, nil
}

func stringifySlice(values []interface{}) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				out = append(out, t)
			}
		default:
			if data, err := json.Marshal(t); err == nil {
				out = append(out, string(data))
			}
		}
	}
	return out
}

func splitToFields(input string) []string {
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == '\n' || r == ',' || r == ';'
	})
	var out []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func applyLoopItem(value interface{}, itemName string, itemValue string) interface{} {
	text, ok := value.(string)
	if !ok {
		return value
	}
	placeholder := fmt.Sprintf("{%s}", itemName)
	return strings.ReplaceAll(text, placeholder, itemValue)
}

func extractFieldFromJSON(jsonText, fieldPath string) (string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		return "", fmt.Errorf("output is not valid JSON: %w", err)
	}

	current := data
	for _, part := range strings.Split(fieldPath, ".") {
		obj, ok := current.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("field %s not found", fieldPath)
		}
		next, exists := obj[part]
		if !exists {
			return "", fmt.Errorf("field %s not found", fieldPath)
		}
		current = next
	}

	switch v := current.(type) {
	case string:
		return v, nil
	case float64, bool, int, int64, uint64:
		return fmt.Sprint(v), nil
	case nil:
		return "", nil
	default:
		marshaled, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(marshaled), nil
	}
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

	// Prefer structured content when present (so JSON results are preserved)
	if result.StructuredContent != nil {
		data, err := json.MarshalIndent(result.StructuredContent, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to encode structured tool result: %w", err)
		}
		if result.IsError {
			return "", errors.New(string(data))
		}
		return string(data), nil
	}

	// Fall back to textual content if no structured content exists
	var parts []string
	for _, content := range result.Content {
		if textContent, ok := mcp.AsTextContent(content); ok {
			parts = append(parts, textContent.Text)
		}
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

// parseTopLevelJSONValues decodes one or more top-level JSON values from the
// provided string. It returns a slice with each decoded value. This handles
// concatenated JSON objects, arrays, and primitive values robustly.
func parseTopLevelJSONValues(s string) ([]interface{}, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var out []interface{}
	for {
		var v interface{}
		if err := dec.Decode(&v); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// WriteCombinedStepOutputs writes aggregated outputs for any workflow steps
// that declare a step-level OutputFilePath. It returns a list of display
// paths (relative to the workflow when possible) for files written.
func WriteCombinedStepOutputs(workflowPath string, wf *Workflow, results map[string]map[string]StepResult) ([]string, error) {
	var written []string
	workflowDir := filepath.Dir(workflowPath)

	for _, step := range wf.Steps {
		if strings.TrimSpace(step.OutputFilePath) == "" {
			continue
		}

		outName := "Result"
		if step.Output != nil && step.Output.Name != "" {
			outName = step.Output.Name
		}
		stepOutputs, ok := results[step.Name]
		if !ok {
			continue
		}
		sr, ok := stepOutputs[outName]
		if !ok || strings.TrimSpace(sr.Value) == "" {
			continue
		}

		combinedPath := ResolveRelativePath(workflowPath, step.OutputFilePath)

		contentToWrite := sr.Value
		if strings.EqualFold(step.Output.Format, "json") || strings.EqualFold(sr.Format, "json") {
			vals, perr := parseTopLevelJSONValues(sr.Value)
			if perr == nil && len(vals) > 0 {
				// Normalize into a single array of items for the `data` field.
				var dataArray []interface{}
				if len(vals) == 1 {
					if arr, ok := vals[0].([]interface{}); ok {
						dataArray = arr
					} else {
						dataArray = []interface{}{vals[0]}
					}
				} else {
					dataArray = vals
				}

				// Inject a computed `package` field (basename without extension)
				for i := range dataArray {
					if obj, ok := dataArray[i].(map[string]interface{}); ok {
						if f, ok := obj["file"].(string); ok && f != "" {
							base := filepath.Base(f)
							name := strings.TrimSuffix(base, filepath.Ext(base))
							obj["package"] = name
							dataArray[i] = obj
						}
					}
				}
				wrapper := map[string]interface{}{"data": dataArray}
				if data, err := json.MarshalIndent(wrapper, "", "  "); err == nil {
					contentToWrite = string(data)
				}
			}
		}

		if err := os.MkdirAll(filepath.Dir(combinedPath), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create combined output dir %s: %v\n", filepath.Dir(combinedPath), err)
			continue
		}
		if err := os.WriteFile(combinedPath, []byte(contentToWrite+"\n"), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write combined output %s: %v\n", combinedPath, err)
			continue
		}
		display := combinedPath
		if rel, relErr := filepath.Rel(workflowDir, combinedPath); relErr == nil && !strings.HasPrefix(rel, "..") {
			display = rel
		}
		written = append(written, display)
	}

	return written, nil
}

// RunFile loads the workflow at the given path and executes it using the
// provided RunnerFunc. The RunnerFunc is responsible for invoking tools
// (handlers) and returning the textual result for each invocation.
func RunFile(ctx context.Context, workflowPath string, runner RunnerFunc) (*Workflow, map[string]map[string]StepResult, error) {
	if runner == nil {
		return nil, nil, errors.New("runner cannot be nil")
	}

	wf, err := LoadFromFile(workflowPath)
	if err != nil {
		return nil, nil, err
	}

	results, err := wf.Execute(ctx, runner)
	if err != nil {
		return wf, nil, err
	}

	return wf, results, nil
}
