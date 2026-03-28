package library

import (
	"fmt"

	"github.com/ronelliott/muzak/library"
	"github.com/ronelliott/snek"
	"github.com/spf13/cobra"
)

func newClearCommand() (*cobra.Command, error) {
	return snek.NewCommand(
		snek.WithUse("clear"),
		snek.WithShort("Remove all sources and clear the track cache"),
		snek.WithSimpleRunE(func(args []string) error {
			sources := library.LoadSources()
			sources.Clear()
			if err := sources.Save(); err != nil {
				return fmt.Errorf("save sources: %w", err)
			}
			fmt.Println("Library cleared")
			return nil
		}),
	)
}
