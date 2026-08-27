package explain

import (
	"strings"
	"testing"
)

const sampleLsMan = `LS(1)                          User Commands                         LS(1)

NAME
       ls - list directory contents

SYNOPSIS
       ls [OPTION]... [FILE]...

DESCRIPTION
       List  information  about  the FILEs (the current directory by default).
       Sort entries alphabetically if none of -cftuvSUX nor --sort  is  speci‐
       fied.

       -a, --all
              do not ignore entries starting with .

       -A, --almost-all
              do not list implied . and ..

       -l     use a long listing format

       -name pattern
              Base of file name matches shell pattern pattern.

AUTHOR
       Written by Richard M. Stallman and David MacKenzie.
`

func TestExtractManSummary(t *testing.T) {
	summary := ExtractManSummary(sampleLsMan)
	want := "ls - list directory contents"
	if summary != want {
		t.Errorf("ExtractManSummary() = %q, want %q", summary, want)
	}
}

func TestFindOptionInMan(t *testing.T) {
	tests := []struct {
		flag string
		want string
	}{
		{
			flag: "-a",
			want: "-a, --all\n              do not ignore entries starting with .",
		},
		{
			flag: "--all",
			want: "-a, --all\n              do not ignore entries starting with .",
		},
		{
			flag: "-l",
			want: "-l     use a long listing format",
		},
		{
			flag: "-name",
			want: "-name pattern\n              Base of file name matches shell pattern pattern.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.flag, func(t *testing.T) {
			got, found := FindOptionInMan(sampleLsMan, tc.flag)
			if !found {
				t.Fatalf("flag %q not found in man", tc.flag)
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("FindOptionInMan(%q) =\n%q\nwant containing:\n%q", tc.flag, got, tc.want)
			}
		})
	}
}
