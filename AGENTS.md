# AGENTS.md - gh-tools

**gh-tools** is a CLI tool for managing GitHub repositories at scale. It provides commands for branch protection, release management, changelog generation, Dependabot configuration, and bulk repository operations. Built with Go using [spf13/cobra](https://github.com/spf13/cobra) for the CLI framework and [go-github](https://github.com/google/go-github) for GitHub API interactions.

## Commands

```bash
make build     # Build binary (uses Docker, outputs to bin/)
make all-build # Build for all platforms
make test      # Run unit tests
make lint      # Run golangci-lint
make fmt       # Format code (uses Docker)
make ci        # Full CI: verify -> check-license -> lint -> build -> unit-tests
make verify    # Verify gen + modules are up to date
make clean     # Clean build artifacts
```

## Development

- After dependency changes: `go mod tidy && go mod vendor`
- License headers required (Apache 2.0): `make add-license`
- Run tests directly: `go test -race -installsuffix "static" ./cmds/... ./internal/...`

## Architecture

- **Entry point**: `main.go` initializes logging via `gomodules.xyz/logs` and executes the root command
- **CLI framework**: spf13/cobra
- **Commands**: `cmds/*.go` with `NewCmd*()` constructor pattern (e.g., `NewCmdRelease()`, `NewCmdProtect()`)
- **Root cmd**: `cmds/root.go` wires all subcommands together
- **Internal packages**: `internal/git/` provides git command wrappers for changelog/release operations
- **Versioning**: Build-time injection via ldflags in `version.go`; version info comes from `gomodules.xyz/x/version`

## Key Patterns

### Command Structure
Each command follows this pattern:
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
Commands handle GitHub rate limits with backoff - see `ListOrgs`, `ListRepos` in `cmds/protect.go` for the pattern using `github.RateLimitError`.

### Sharding
Bulk operations support sharding via `--shards` and `--shard-index` flags for parallel execution across multiple runners.

## Testing

- Uses `github.com/stretchr/testify`
- Test files: `*_test.go`

## Adding New Commands

1. Create `cmds/new_command.go` following the `NewCmd*` pattern
2. Register in `cmds/root.go`: `cmd.AddCommand(NewCmdNewCommand())`
3. Use `gomodules.xyz/flags.PrintFlags()` in `PersistentPreRun` for debugging
4. For GitHub API operations, follow the authentication pattern from existing commands

## CI

- Go 1.25
- Runs on: ubuntu-24.04
- Runs `make ci` on PRs and push to master