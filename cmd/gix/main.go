package main

import (
	"context"
	"fmt"
	"os"

	"gix/internal/app"
	"gix/internal/i18n"
)

func main() {
	application := app.New(os.Stdin, os.Stdout, os.Stderr)
	if err := application.Run(context.Background(), os.Args[1:]); err != nil {
		catalog := i18n.NewCatalog(i18n.Detect())
		fmt.Fprintln(os.Stderr, catalog.S("common.error_prefix"), i18n.LocalizeError(err, catalog))
		os.Exit(1)
	}
}
