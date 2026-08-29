package explain

import (
	"strings"
	"testing"
)

const sampleLsHelp = `Usage: ls [OPTION]... [FILE]...
List information about the FILEs (the current directory by default).
Sort entries alphabetically if none of -cftuvSUX nor --sort is specified.

Mandatory arguments to long options are mandatory for short options too.
  -a, --all                  do not ignore entries starting with .
  -A, --almost-all           do not list implied . and ..
      --author               with -l, print the author of each file
  -b, --escape               print C-style escapes for nongraphic characters
      --block-size=SIZE      with -l, scale sizes by SIZE when printing them;
                             e.g., '--block-size=M'; see SIZE format below

  -B, --ignore-backups       do not list implied entries ending with ~
  -c                         with -lt: sort by, and show, ctime
  -C                         list entries by columns
      --color[=WHEN]         color the output WHEN; more info below
  -l                         use a long listing format
`

func TestFindOptionInHelp(t *testing.T) {
	tests := []struct {
		flag string
		want string
	}{
		{
			flag: "-a",
			want: "-a, --all                  do not ignore entries starting with .",
		},
		{
			flag: "--all",
			want: "-a, --all                  do not ignore entries starting with .",
		},
		{
			flag: "-l",
			want: "-l                         use a long listing format",
		},
		{
			flag: "--author",
			want: "--author               with -l, print the author of each file",
		},
		{
			flag: "--color",
			want: "--color[=WHEN]         color the output WHEN; more info below",
		},
		{
			flag: "--block-size",
			want: "--block-size=SIZE      with -l, scale sizes by SIZE when printing them;\n                             e.g., '--block-size=M'; see SIZE format below",
		},
	}

	for _, tc := range tests {
		t.Run(tc.flag, func(t *testing.T) {
			got, found := FindOptionInHelp(sampleLsHelp, tc.flag)
			if !found {
				t.Fatalf("flag %q not found in help", tc.flag)
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("FindOptionInHelp(%q) =\n%q\nwant containing:\n%q", tc.flag, got, tc.want)
			}
		})
	}
}

func TestFindOptionInHelpNotFound(t *testing.T) {
	_, found := FindOptionInHelp(sampleLsHelp, "--nonexistent")
	if found {
		t.Errorf("expected --nonexistent not to be found")
	}
}
