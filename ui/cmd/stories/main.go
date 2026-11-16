// Command stories shows pain's UI components on their own, to try them out:
//
//	go run ./ui/cmd/stories
package main

import (
	"fmt"
	"os"

	"pain/ui/hold"
	"pain/ui/story"
)

func main() {
	if err := story.Run(hold.Stories); err != nil {
		fmt.Fprintln(os.Stderr, "stories:", err)
		os.Exit(1)
	}
}
