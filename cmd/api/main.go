// 06
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rshdhere/bookmyShow/internal/server"
)

func main() {
	ctx := context.Background()
	if err := server.Run(ctx, os.Getenv, os.Stderr); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
