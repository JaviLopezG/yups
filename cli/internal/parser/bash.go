package parser

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

func ExtractCommands(input string) ([]string, error) {

	p := syntax.NewParser()
	file, err := p.Parse(strings.NewReader(input), "")
	if err != nil {
		return nil, err
	}

	var commands []string

	syntax.Walk(file, func(node syntax.Node) bool {
		if call, ok := node.(*syntax.CallExpr); ok {
			if len(call.Args) == 0 {
				return true
			}

			cmdName := call.Args[0].Lit()
			if cmdName != "" {
				//TODO save arguments to allow normalization
				commands = append(commands, cmdName)
			}
		}
		return true
	})

	return commands, nil
}

func NormalizeCommand(cmdList []string) []string {
	launchers := map[string]bool{
		"sudo": true, "doas": true, "env": true,
		"nohup": true, "nice": true, "time": true,
		"watch": true, "xargs": true, "timeout": true,
		"runcon": true, "setpriv": true, "stdbuf": true,
		"dbus-run-session": true, "exec": true,
		"bash": true, "sh": true,
	}

	var cleaned []string
	for _, cmd := range cmdList {
		// TODO look arguments of parent node
		if !launchers[cmd] {
			cleaned = append(cleaned, cmd)
		}
	}
	return cleaned
}
