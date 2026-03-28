// Package library provides the "muzak library" subcommand and its children.
package library

import (
	"github.com/ronelliott/snek"
	"github.com/spf13/cobra"
)

// NewCommand returns the "muzak library" parent command.
func NewCommand() (*cobra.Command, error) {
	return snek.NewCommand(
		snek.WithUse("library"),
		snek.WithShort("Manage the music library"),
		snek.WithSubCommandGenerator(newAddCommand),
		snek.WithSubCommandGenerator(newRmCommand),
		snek.WithSubCommandGenerator(newClearCommand),
		snek.WithSubCommandGenerator(newScanCommand),
	)
}
