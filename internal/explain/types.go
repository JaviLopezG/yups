package explain

// PipelineExplanation holds the structured explanation of an entire pipeline.
type PipelineExplanation struct {
	Stages []StageExplanation
}

// StageExplanation holds the explanation of one pipeline stage and its connector.
type StageExplanation struct {
	Command   *CommandExplanation
	Operator  ControlOperator
	OpSummary string
}

// CommandExplanation holds the structured documentation for a single command.
type CommandExplanation struct {
	Name             string
	Subcommand       string
	EnvVars          []string
	AliasInfo        string
	BuiltinInfo      string
	Summary          string
	Found            bool
	Wrappers         []WrapperExplanation
	Flags            []FlagExplanation
	PositionalArgs   []ArgExplanation
	Redirects        []RedirectExplanation
	LLMQueried       bool
	LLMEndpoint      string
	LLMExplanation   string
	SuggestedCommand string
	SuggestedScript  string
	LLMError         string
}

// WrapperExplanation holds documentation for a command wrapper (e.g. sudo).
type WrapperExplanation struct {
	Name    string
	Summary string
	Flags   []FlagExplanation
	Args    []string
}

// FlagExplanation holds documentation for a single flag.
type FlagExplanation struct {
	Flag        Flag
	Description string
	Found       bool
	Source      string // "help" or "man"
}

// ArgExplanation describes a positional argument and its filesystem nature.
type ArgExplanation struct {
	Value string
	Kind  string // "directory", "file", "argument"
}

// RedirectExplanation describes an I/O redirection.
type RedirectExplanation struct {
	Op          string
	Target      string
	Description string
}
