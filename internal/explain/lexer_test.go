package explain

import (
	"reflect"
	"testing"
)

func TestTokenizeSimple(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []Token
	}{
		{
			name: "single command with flags",
			args: []string{"ls", "-al", "/var/cache/man/"},
			want: []Token{
				{Type: TokenWord, Value: "ls"},
				{Type: TokenWord, Value: "-al"},
				{Type: TokenWord, Value: "/var/cache/man/"},
			},
		},
		{
			name: "single string with compound command",
			args: []string{"ls -al /var/cache || ps aux"},
			want: []Token{
				{Type: TokenWord, Value: "ls"},
				{Type: TokenWord, Value: "-al"},
				{Type: TokenWord, Value: "/var/cache"},
				{Type: TokenOpOr, Value: "||"},
				{Type: TokenWord, Value: "ps"},
				{Type: TokenWord, Value: "aux"},
			},
		},
		{
			name: "pipe and redirect",
			args: []string{"cat file.txt | grep foo > out.txt 2>&1"},
			want: []Token{
				{Type: TokenWord, Value: "cat"},
				{Type: TokenWord, Value: "file.txt"},
				{Type: TokenOpPipe, Value: "|"},
				{Type: TokenWord, Value: "grep"},
				{Type: TokenWord, Value: "foo"},
				{Type: TokenRedir, Value: ">"},
				{Type: TokenWord, Value: "out.txt"},
				{Type: TokenRedir, Value: "2>&1"},
			},
		},
		{
			name: "quotes preserved in args",
			args: []string{"git", "commit", "-m", "initial commit"},
			want: []Token{
				{Type: TokenWord, Value: "git"},
				{Type: TokenWord, Value: "commit"},
				{Type: TokenWord, Value: "-m"},
				{Type: TokenWord, Value: "initial commit"},
			},
		},
		{
			name: "and operator with semicolon",
			args: []string{"mkdir -p /tmp/test && cd /tmp/test ; ls"},
			want: []Token{
				{Type: TokenWord, Value: "mkdir"},
				{Type: TokenWord, Value: "-p"},
				{Type: TokenWord, Value: "/tmp/test"},
				{Type: TokenOpAnd, Value: "&&"},
				{Type: TokenWord, Value: "cd"},
				{Type: TokenWord, Value: "/tmp/test"},
				{Type: TokenOpSemi, Value: ";"},
				{Type: TokenWord, Value: "ls"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Tokenize(tc.args)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Tokenize(%v) =\n%#v\nwant:\n%#v", tc.args, got, tc.want)
			}
		})
	}
}

func TestTokenizeNaturalLanguageQueries(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []Token
	}{
		{
			name: "question in spanish starting with question mark",
			args: []string{"¿cómo puedo ver los puertos abiertos?"},
			want: []Token{
				{Type: TokenComment, Value: "¿cómo puedo ver los puertos abiertos?"},
			},
		},
		{
			name: "question in spanish without initial question mark",
			args: []string{"como", "ver", "la", "memoria", "libre"},
			want: []Token{
				{Type: TokenComment, Value: "como ver la memoria libre"},
			},
		},
		{
			name: "question in english starting with how to",
			args: []string{"how to find large files in /home"},
			want: []Token{
				{Type: TokenComment, Value: "how to find large files in /home"},
			},
		},
		{
			name: "command with comment is not treated as pure natural language",
			args: []string{"ls -a # como listar"},
			want: []Token{
				{Type: TokenWord, Value: "ls"},
				{Type: TokenWord, Value: "-a"},
				{Type: TokenComment, Value: "como listar"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Tokenize(tc.args)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Tokenize(%v) =\n%#v\nwant:\n%#v", tc.args, got, tc.want)
			}
		})
	}
}
