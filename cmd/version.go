package cmd

import (
	"fmt"
)

// VersionCmd is the "version" command.
type VersionCmd struct{}

// Execute the 'version' commands.
func (cmd *VersionCmd) Execute([]string) error {
	fmt.Printf("Beaver %s \nBuild Date: %s\nCommit SHA: %s\n", Version, BuildDate, CommitSha)

	return nil
}

func init() {
	if _, err := parser.AddCommand("version", "Print the program version", "", &VersionCmd{}); err != nil {
		Logger.Fatal().Err(err).Msg("error adding command")
	}
}
