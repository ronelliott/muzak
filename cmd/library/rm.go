package library

import (
	"fmt"

	"github.com/ronelliott/muzak/library"
	"github.com/ronelliott/snek"
	"github.com/spf13/cobra"
)

func newRmCommand() (*cobra.Command, error) {
	return snek.NewCommand(
		snek.WithUse("rm <path>"),
		snek.WithShort("Remove a source directory from the library"),
		snek.WithSimpleRunE(func(args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("expected exactly one path argument")
			}

			sources := library.LoadSources()
			if err := sources.Remove(args[0]); err != nil {
				return err
			}
			if err := sources.Save(); err != nil {
				return fmt.Errorf("save sources: %w", err)
			}

			fmt.Printf("Removed %s\n", args[0])
			return nil
		}),
	)
}
