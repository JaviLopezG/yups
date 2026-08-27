package explain

import (
	"context"
	"io"
)

// Explain parses and explains the provided command line arguments, writing
// the result to stdout.
func Explain(ctx context.Context, env DocEnv, args []string, stdout, stderr io.Writer, color bool) int {
	if len(args) == 0 {
		return 0
	}

	pipeline := Parse(args)
	if len(pipeline.Stages) == 0 {
		return 0
	}

	resolver := NewResolver(env)
	exp := resolver.ExplainPipeline(ctx, pipeline)

	FormatPipeline(stdout, exp, FormatOptions{Color: color})
	return 0
}
