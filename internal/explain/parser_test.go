package explain

import (
	"testing"
)

func TestParseSimpleCommand(t *testing.T) {
	pipeline := Parse([]string{"ls", "-al", "/var/cache/man/"})
	if len(pipeline.Stages) != 1 {
		t.Fatalf("stages count = %d, want 1", len(pipeline.Stages))
	}
	stage := pipeline.Stages[0]
	if stage.Command.Name != "ls" {
		t.Errorf("command name = %q, want 'ls'", stage.Command.Name)
	}
	if len(stage.Command.Flags) != 2 {
		t.Fatalf("flags count = %d, want 2 (-a, -l)", len(stage.Command.Flags))
	}
	if stage.Command.Flags[0].Name != "-a" || stage.Command.Flags[1].Name != "-l" {
		t.Errorf("flags = [%s, %s], want [-a, -l]", stage.Command.Flags[0].Name, stage.Command.Flags[1].Name)
	}
	if len(stage.Command.Args) != 1 || stage.Command.Args[0] != "/var/cache/man/" {
		t.Errorf("args = %v, want ['/var/cache/man/']", stage.Command.Args)
	}
}

func TestParseLongFlags(t *testing.T) {
	pipeline := Parse([]string{"ls", "--all", "--color=auto", "-h"})
	if len(pipeline.Stages) != 1 {
		t.Fatalf("stages count = %d, want 1", len(pipeline.Stages))
	}
	flags := pipeline.Stages[0].Command.Flags
	if len(flags) != 3 {
		t.Fatalf("flags count = %d, want 3", len(flags))
	}
	if flags[0].Name != "--all" || flags[0].Value != "" {
		t.Errorf("flag 0 = %+v, want --all without value", flags[0])
	}
	if flags[1].Name != "--color" || flags[1].Value != "auto" {
		t.Errorf("flag 1 = %+v, want --color with value 'auto'", flags[1])
	}
	if flags[2].Name != "-h" {
		t.Errorf("flag 2 = %+v, want -h", flags[2])
	}
}

func TestParseCompoundPipeline(t *testing.T) {
	pipeline := Parse([]string{"ls", "-l", "||", "echo", "failed"})
	if len(pipeline.Stages) != 2 {
		t.Fatalf("stages count = %d, want 2", len(pipeline.Stages))
	}
	if pipeline.Stages[0].Operator != OpOr {
		t.Errorf("stage 0 operator = %q, want %q", pipeline.Stages[0].Operator, OpOr)
	}
	if pipeline.Stages[0].Command.Name != "ls" {
		t.Errorf("stage 0 command = %q, want 'ls'", pipeline.Stages[0].Command.Name)
	}
	if pipeline.Stages[1].Command.Name != "echo" {
		t.Errorf("stage 1 command = %q, want 'echo'", pipeline.Stages[1].Command.Name)
	}
	if len(pipeline.Stages[1].Command.Args) != 1 || pipeline.Stages[1].Command.Args[0] != "failed" {
		t.Errorf("stage 1 args = %v, want ['failed']", pipeline.Stages[1].Command.Args)
	}
}

func TestParseWrapperAndEnvVars(t *testing.T) {
	pipeline := Parse([]string{"FOO=1", "BAR=baz", "sudo", "-u", "root", "grep", "-i", "error", "/var/log/syslog"})
	if len(pipeline.Stages) != 1 {
		t.Fatalf("stages count = %d, want 1", len(pipeline.Stages))
	}
	cmd := pipeline.Stages[0].Command
	if len(cmd.EnvVars) != 2 || cmd.EnvVars[0] != "FOO=1" || cmd.EnvVars[1] != "BAR=baz" {
		t.Errorf("env vars = %v, want ['FOO=1', 'BAR=baz']", cmd.EnvVars)
	}
	if len(cmd.Wrappers) != 1 || cmd.Wrappers[0].Name != "sudo" {
		t.Fatalf("wrappers = %v, want 1 sudo wrapper", cmd.Wrappers)
	}
	if len(cmd.Wrappers[0].Args) != 1 || cmd.Wrappers[0].Args[0] != "root" {
		t.Errorf("wrapper args = %v, want ['root']", cmd.Wrappers[0].Args)
	}
	if cmd.Name != "grep" {
		t.Errorf("command name = %q, want 'grep'", cmd.Name)
	}
	if len(cmd.Flags) != 1 || cmd.Flags[0].Name != "-i" {
		t.Errorf("flags = %v, want ['-i']", cmd.Flags)
	}
	if len(cmd.Args) != 2 || cmd.Args[0] != "error" || cmd.Args[1] != "/var/log/syslog" {
		t.Errorf("args = %v, want ['error', '/var/log/syslog']", cmd.Args)
	}
}

func TestParseSubcommand(t *testing.T) {
	pipeline := Parse([]string{"git", "commit", "-m", "fix bug"})
	if len(pipeline.Stages) != 1 {
		t.Fatalf("stages count = %d, want 1", len(pipeline.Stages))
	}
	cmd := pipeline.Stages[0].Command
	if cmd.Name != "git" {
		t.Errorf("command name = %q, want 'git'", cmd.Name)
	}
	if cmd.Subcommand != "commit" {
		t.Errorf("subcommand = %q, want 'commit'", cmd.Subcommand)
	}
	if len(cmd.Flags) != 1 || cmd.Flags[0].Name != "-m" {
		t.Errorf("flags = %v, want ['-m']", cmd.Flags)
	}
	if len(cmd.Args) != 1 || cmd.Args[0] != "fix bug" {
		t.Errorf("args = %v, want ['fix bug']", cmd.Args)
	}
}

func TestFormatRawPipelineCommentNoDuplication(t *testing.T) {
	t.Run("pure-comment", func(t *testing.T) {
		p := Parse([]string{"#helo"})
		raw := formatRawPipeline(p)
		want := "# helo"
		if raw != want {
			t.Errorf("formatRawPipeline(#helo) = %q, want %q", raw, want)
		}
	})

	t.Run("command-with-comment", func(t *testing.T) {
		p := Parse([]string{"ls", ".yups", "#I", "want", "to", "ls", "recursively"})
		raw := formatRawPipeline(p)
		want := "ls .yups # I want to ls recursively"
		if raw != want {
			t.Errorf("formatRawPipeline(ls .yups #...) = %q, want %q", raw, want)
		}
	})
}

func TestParseComplexPipelineAndRedirects(t *testing.T) {
	pipeline := Parse([]string{"cat file.txt | grep foo > out.txt 2>&1"})
	if len(pipeline.Stages) != 2 {
		t.Fatalf("stages count = %d, want 2", len(pipeline.Stages))
	}
	if pipeline.Stages[0].Command.Name != "cat" || pipeline.Stages[0].Operator != OpPipe {
		t.Errorf("stage 0: cmd=%q, op=%q, want 'cat', '|'", pipeline.Stages[0].Command.Name, pipeline.Stages[0].Operator)
	}
	if pipeline.Stages[1].Command.Name != "grep" {
		t.Errorf("stage 1: cmd=%q, want 'grep'", pipeline.Stages[1].Command.Name)
	}
	if len(pipeline.Stages[1].Command.Redirects) != 2 {
		t.Fatalf("redirects count = %d, want 2", len(pipeline.Stages[1].Command.Redirects))
	}
	if pipeline.Stages[1].Command.Redirects[0].Op != ">" || pipeline.Stages[1].Command.Redirects[0].Target != "out.txt" {
		t.Errorf("redir 0 = %+v", pipeline.Stages[1].Command.Redirects[0])
	}
	if pipeline.Stages[1].Command.Redirects[1].Op != "2>&1" {
		t.Errorf("redir 1 = %+v", pipeline.Stages[1].Command.Redirects[1])
	}
}

func TestParseSemicolonAndBackground(t *testing.T) {
	pipeline := Parse([]string{"mkdir -p /tmp/test && cd /tmp/test ; ls"})
	if len(pipeline.Stages) != 3 {
		t.Fatalf("stages count = %d, want 3", len(pipeline.Stages))
	}
	if pipeline.Stages[0].Command.Name != "mkdir" || pipeline.Stages[0].Operator != OpAnd {
		t.Errorf("stage 0: cmd=%q, op=%q, want 'mkdir', '&&'", pipeline.Stages[0].Command.Name, pipeline.Stages[0].Operator)
	}
	if pipeline.Stages[1].Command.Name != "cd" || pipeline.Stages[1].Operator != OpSemicolon {
		t.Errorf("stage 1: cmd=%q, op=%q, want 'cd', ';'", pipeline.Stages[1].Command.Name, pipeline.Stages[1].Operator)
	}
	if pipeline.Stages[2].Command.Name != "ls" || pipeline.Stages[2].Operator != OpNone {
		t.Errorf("stage 2: cmd=%q, op=%q, want 'ls', ''", pipeline.Stages[2].Command.Name, pipeline.Stages[2].Operator)
	}
}

func TestParseNaturalLanguageQueries(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "spanish question",
			args: []string{"¿cómo puedo ver los puertos abiertos?"},
			want: "¿cómo puedo ver los puertos abiertos?",
		},
		{
			name: "spanish question split words",
			args: []string{"como", "ver", "la", "memoria", "libre"},
			want: "como ver la memoria libre",
		},
		{
			name: "english question",
			args: []string{"how to find large files in /home"},
			want: "how to find large files in /home",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parse(tt.args)
			if p.Comment != tt.want {
				t.Errorf("Parse(%v).Comment = %q, want %q", tt.args, p.Comment, tt.want)
			}
			if len(p.Stages) != 0 {
				t.Errorf("Parse(%v).Stages = %d, want 0", tt.args, len(p.Stages))
			}
		})
	}
}
