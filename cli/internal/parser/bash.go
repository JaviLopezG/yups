package parser

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

var launchers = map[string]bool{
	"sudo": true, "doas": true, "env": true, "nohup": true,
	"nice": true, "time": true, "watch": true, "xargs": true,
	"timeout": true, "runcon": true, "setpriv": true, "stdbuf": true,
	"exec": true, "bash": true, "sh": true, "strace": true,
}

func ExtractEffectiveCommand(input string) (string, error) {
	p := syntax.NewParser()
	file, err := p.Parse(strings.NewReader(input), "")
	if err != nil {
		return "", err
	}

	var effectiveCmd string

	syntax.Walk(file, func(node syntax.Node) bool {
		if call, ok := node.(*syntax.CallExpr); ok {
			for _, arg := range call.Args {
				lit := arg.Lit()
				if !launchers[lit] &&
					!strings.Contains(lit, "=") &&
					!strings.HasPrefix(lit, "-") {
					effectiveCmd = clean(lit)
					return false
				}
			}
		}
		return true
	})

	return effectiveCmd, nil
}

func clean(lit string) string {
	if strings.HasPrefix(lit, ".") ||
		strings.HasPrefix(lit, "/") {
		subs := strings.Split(lit, "/")
		return subs[len(subs)-1]
	} else {
		return lit
	}
}
