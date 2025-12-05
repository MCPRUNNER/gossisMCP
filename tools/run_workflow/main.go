package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	packagehandlers "github.com/MCPRUNNER/gossisMCP/pkg/handlers/packages"
	templatehandlers "github.com/MCPRUNNER/gossisMCP/pkg/handlers/templates"
	"github.com/MCPRUNNER/gossisMCP/pkg/workflow"
	"github.com/mark3labs/mcp-go/mcp"
)

func main() {
	ctx := context.Background()
	workflowPath := ".gossismcp/workflows/workflow_example_loop.json"
	packageDir := ".gossismcp"
	absPath, err := filepath.Abs(workflowPath)
	if err == nil {
		workflowPath = absPath
	}
	if pkgAbs, err := filepath.Abs(packageDir); err == nil {
		packageDir = pkgAbs
	}

	runner := func(stepCtx context.Context, tool string, params map[string]interface{}) (string, error) {
		req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: params}}
		switch tool {
		case "list_packages":
			res, err := packagehandlers.HandleListPackages(stepCtx, req, "", "")
			if err != nil {
				return "", err
			}
			return workflow.ToolResultToString(res)
		case "analyze_logging_configuration":
			res, err := packagehandlers.HandleAnalyzeLoggingConfiguration(stepCtx, req, "")
			if err != nil {
				return "", err
			}
			return workflow.ToolResultToString(res)
		case "render_template":
			res, err := templatehandlers.HandleRenderTemplate(stepCtx, req, packageDir)
			if err != nil {
				return "", err
			}
			return workflow.ToolResultToString(res)
		default:
			return "", fmt.Errorf("unsupported tool: %s", tool)
		}
	}

	wf, results, err := workflow.RunFile(ctx, workflowPath, runner)
	if err != nil {
		fmt.Fprintf(os.Stderr, "workflow run failed: %v\n", err)
		os.Exit(1)
	}

	written, err := workflow.WriteCombinedStepOutputs(workflowPath, wf, results)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed writing combined outputs: %v\n", err)
	}

	fmt.Println("Per-step combined files written:")
	for _, p := range written {
		fmt.Println(" -", p)
	}

	fmt.Println("Done")
}
