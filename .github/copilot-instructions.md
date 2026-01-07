# Copilot Instructions for gh-tools

## Project Overview

**gh-tools** is a CLI tool for managing GitHub repositories at scale. It provides commands for branch protection, release management, changelog generation, Dependabot configuration, and bulk repository operations. Built with Go using [spf13/cobra](https://github.com/spf13/cobra) for the CLI framework and [go-github](https://github.com/google/go-github) for GitHub API interactions.

## Architecture

- **Entry point**: [main.go](../main.go) initializes logging via `gomodules.xyz/logs` and executes the root command
- **Commands**: All CLI commands live in [cmds/](../cmds/) with `NewCmd*` constructor pattern (e.g., `NewCmdRelease()`, `NewCmdProtect()`)
- **Root command**: [cmds/root.go](../cmds/root.go) wires all subcommands together
- **Internal packages**: [internal/git/](../internal/git/) provides git command wrappers for changelog/release operations
- **Versioning**: Build-time injection via ldflags in [version.go](../version.go); version info comes from `gomodules.xyz/x/version`

## Key Patterns

### Command Structure
Each command follows this pattern in [cmds/](../cmds/):
```go
func NewCmdExample() *cobra.Command {
    var flagVar string
    cmd := &cobra.Command{
        Use:   "example",
        Short: "description",
        PersistentPreRun: func(c *cobra.Command, args []string) {
            flags.PrintFlags(c.Flags())  // Always print flags for debugging
        },
        Run: func(cmd *cobra.Command, args []string) {
            runExample(flagVar)
        },
    }
    cmd.Flags().StringVar(&flagVar, "flag-name", "", "description")
    return cmd
}
```

### GitHub API Authentication
All commands requiring GitHub API access expect `GH_TOOLS_TOKEN` environment variable:
```go
token, found := os.LookupEnv("GH_TOOLS_TOKEN")
if !found {
    log.Fatalln("GH_TOOLS_TOKEN env var is not set")
}
```

### Rate Limiting
Commands handle GitHub rate limits with backoff - see `ListOrgs`, `ListRepos` in [cmds/protect.go](../cmds/protect.go) for the pattern using `github.RateLimitError`.

### Sharding
Bulk operations support sharding via `--shards` and `--shard-index` flags for parallel execution across multiple runners.

## Build & Development

```bash
# Build for current platform (uses Docker)
make build

# Build for all platforms
make all-build

# Format code
make fmt

# Run tests
make test

# Run linter
make lint

# Full CI check
make ci

# Clean build artifacts
make clean
```

**Vendoring**: This project uses `go mod vendor`. After modifying dependencies:
```bash
go mod tidy && go mod vendor
```

## Testing

- Tests use `github.com/stretchr/testify` for assertions
- Test files follow `*_test.go` convention (see [internal/git/git_test.go](../internal/git/git_test.go))
- Run with `make test` or directly: `go test -race ./cmds/... ./internal/...`

## Adding New Commands

1. Create `cmds/new_command.go` following the `NewCmd*` pattern
2. Register in [cmds/root.go](../cmds/root.go): `cmd.AddCommand(NewCmdNewCommand())`
3. Use `gomodules.xyz/flags.PrintFlags()` in `PersistentPreRun` for debugging
4. For GitHub API operations, follow the authentication pattern from existing commands

## License

All files require Apache 2.0 license header. Run `make add-license` to add headers automatically.
