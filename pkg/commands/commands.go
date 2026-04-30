//
// Copyright (c) 2026 Red Hat, Inc.
// Licensed under the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package commands

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type SubCommandType string

const (
	Command = "/generate-che-doc"

	WelcomeMarker = "<!-- che-doc-generator-welcome -->"

	SubCommandDefault SubCommandType = ""
	SubCommandHelp    SubCommandType = "help"
)

type SubCommand struct {
	Type        SubCommandType
	Description string
}

var (
	parsePattern = regexp.MustCompile(`(?:^|\s)` + regexp.QuoteMeta(Command) + `(?:[ \t]+(\S+))?(?:[ \t]|$|\n|\r)`)

	SubCommands = []SubCommand{
		{Type: SubCommandDefault, Description: "Generate a documentation PR based on this PR's changes"},
		{Type: SubCommandHelp, Description: "Show this help message"},
	}
)

func BuildWelcomeMessage(pollInterval time.Duration) string {
	var b strings.Builder

	b.WriteString(WelcomeMarker)
	b.WriteString("\n")
	b.WriteString("Hi! I'm **che-doc-generator** — I help create documentation PRs from code changes.\n\n")
	b.WriteString(fmt.Sprintf("I check for new commands every **%s**.\n\n", pollInterval))
	b.WriteString("**Available commands**:\n")

	for _, subCommand := range SubCommands {
		b.WriteString("- `" + Command + " " + string(subCommand.Type) + "` — " + subCommand.Description + "\n")
	}

	return b.String()
}

// Parse extracts a subcommand from a comment body containing the trigger prefix.
// Returns the subcommand name and true if the prefix is found, or false otherwise.
func Parse(body string) (bool, SubCommandType) {
	items := parsePattern.FindStringSubmatch(body)
	if items == nil {
		return false, ""
	}

	return true, SubCommandType(items[1])
}
