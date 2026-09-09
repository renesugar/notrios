// Command docgen deterministically renders anchored documentation subsets.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/docgen"
)

func main() {
	root := flag.String("root", ".", "repository root")
	check := flag.Bool("check", false, "check freshness without writing")
	user := flag.Bool("user", false, "generate user documentation")
	api := flag.Bool("api", false, "generate API documentation")
	flag.Parse()
	if !*user && !*api {
		fail("at least one of --user or --api is required")
	}
	g := docgen.Generator{Resolver: docgen.RepositoryResolver(*root)}
	for _, audience := range []struct {
		name    string
		enabled bool
	}{{"user", *user}, {"api", *api}} {
		if !audience.enabled {
			continue
		}
		if *check {
			drift, err := g.Check(*root, audience.name)
			if err != nil {
				fail(err.Error())
			}
			if len(drift) > 0 {
				fail("stale generated documentation: " + strings.Join(drift, ", "))
			}
		} else if err := g.Write(*root, audience.name); err != nil {
			fail(err.Error())
		}
	}
}
func fail(message string) { fmt.Fprintln(os.Stderr, "docgen:", message); os.Exit(1) }
