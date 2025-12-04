# MCP Configuration Examples

This guide shows several ways to configure `mcp.json` so the SSIS Analyzer MCP server can be launched with different runtime settings.

## Configuration Sources Recap

- `mcp.json` tells the MCP client how to start the analyzer (`stdio` command) or where to find an already running HTTP endpoint.
- Command-line flags include `--config <path>`, `--pkg-dir <path>`, `--http`, and `--port <value>`.
- Environment variables override parts of the configuration when the process starts:
  - `GOSSIS_HTTP_PORT`: replaces `server.port`.
  - `GOSSIS_PKG_DIRECTORY`: replaces `packages.directory`.
  - `GOSSIS_LOG_LEVEL`: replaces `logging.level` (`debug`, `info`, `warn`, `error`).
  - `GOSSIS_LOG_FORMAT`: replaces `logging.format` (`text`, `json`).
- Config files (`config.json` / `config.yaml`) support:
  - `server.http_mode` (bool) and `server.port` (string).
  - `packages.directory` and `packages.exclude_file` (relative to the package directory when not absolute).
  - `logging.level` and `logging.format`.
  - Optional `plugins` section (directory, enabled plugin IDs, registry, update policy, security settings).

## Example 1 – Minimal stdio launch

Use compiled binary with default settings. The analyzer falls back to `config.json`/`config.yaml` in the working directory or built-in defaults.

```jsonc
{
  "servers": {
    "ssis-analyzer": {
      "type": "stdio",
      "command": ".\\ssis-analyzer.exe"
    }
  }
}
```

## Example 2 – Stdio with explicit config file and overrides

Reference a shared YAML config that sets `packages.directory`, `packages.exclude_file`, and plugin preferences. Environment variables tighten logging.

```jsonc
{
  "servers": {
    "ssis-analyzer": {
      "type": "stdio",
      "command": ".\\ssis-analyzer.exe",
      "args": ["--config", ".\\config.yaml"],
      "env": {
        "GOSSIS_LOG_LEVEL": "warn",
        "GOSSIS_LOG_FORMAT": "json"
      },
      "cwd": "."
    }
  }
}
```

## Example 3 – Stdio launch with dynamic package workspace

Select the SSIS project directory at runtime via `GOSSIS_PKG_DIRECTORY`. Optionally set a custom ignore file path in `config.yaml` if it is outside the package root.

```jsonc
{
  "servers": {
    "ssis-analyzer": {
      "type": "stdio",
      "command": ".\\ssis-analyzer.exe",
      "args": ["--config", ".\\config.yaml"],
      "env": {
        "GOSSIS_PKG_DIRECTORY": "C:\\Data\\ssis\\Production",
        "GOSSIS_HTTP_PORT": "8090"
      }
    }
  }
}
```

`packages.exclude_file` in `config.yaml` can point to a `.gossisignore` file (absolute or relative to `packages.directory`) to filter build outputs (`bin/`, `obj/`, `*.ispac`, etc.).

## Example 4 – HTTP mode with shared daemon

Run the analyzer once as an HTTP server, then have clients connect via `mcp.json`. Start the daemon separately:

```powershell
.\\ssis-analyzer.exe --http --port 9090 --pkg-dir C:\\Data\\ssis\\Sandbox
```

Client configuration:

```jsonc
{
  "servers": {
    "ssis-analyzer-http": {
      "type": "http",
      "url": "http://localhost:9090/mcp"
    }
  }
}
```

In HTTP mode, use a config file to declare `packages.exclude_file` if you need ignore patterns; environment variables still override the in-process configuration.

## Example 5 – Multiple profiles in one file

Combine both stdio and HTTP entries so developers can switch between local execution and a remote analyzer.

```jsonc
{
  "servers": {
    "ssis-analyzer-stdio": {
      "type": "stdio",
      "command": ".\\ssis-analyzer.exe",
      "env": {
        "GOSSIS_PKG_DIRECTORY": ".\\Documents\\SSIS_EXAMPLES"
      }
    },
    "ssis-analyzer-remote": {
      "type": "http",
      "url": "https://ssis-analyzer.example.com/mcp"
    }
  }
}
```

Each profile can point at a different config file, package folder, or logging policy, letting you adapt the analyzer to team workflows without editing the binary.

## Example 6 – Launch from source with arguments array

Useful when distributing the project without a compiled binary. The `command` selects the executable (`go` in this case), while `args` supply the launch parameters for the analyzer.

```jsonc
{
  "servers": {
    "ssis-analyzer-dev": {
      "type": "stdio",
      "command": "go",
      "args": [
        "run",
        ".",
        "--config",
        ".\\dev-config.yaml",
        "--pkg-dir",
        "C:\\Data\\ssis\\Dev"
      ],
      "env": {
        "GOSSIS_LOG_LEVEL": "debug"
      }
    }
  }
}
```

`args` may also be combined with scripts or wrapper executables (for example, `powershell` plus script name) when teams need pre-launch setup steps.
