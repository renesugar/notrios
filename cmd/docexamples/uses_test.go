package main

import (
	"strings"
	"testing"
)

// J15: a recipe's declaration is checked against the surfaces that define them,
// and against its own steps. A fence cannot fail when a command it documents
// stops existing; this is what can.

func j15Surfaces(t *testing.T) surfaces {
	t.Helper()
	known, err := loadSurfaces(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	return known
}

func recipe(steps ...string) Example {
	example := Example{Section: "s", Ordinal: 1, Kind: KindRecipe}
	for _, step := range steps {
		example.Steps = append(example.Steps, Step{Run: step})
	}
	return example
}

func problems(t *testing.T, example Example) string {
	t.Helper()
	found := []string{}
	for _, problem := range j15Surfaces(t).checkUses("example", example) {
		found = append(found, problem.Error())
	}
	return strings.Join(found, "\n")
}

func TestJ15ADeclarationMustNameSomethingReal(t *testing.T) {
	example := recipe("curl -s http://127.0.0.1:8080/api/v1/status | jq")
	example.Uses.REST = []string{"GET /api/v1/status"}
	if got := problems(t, example); got != "" {
		t.Errorf("a route the server registers and the recipe calls: %s", got)
	}

	example.Uses.REST = []string{"GET /api/v1/there-is-no-such-route"}
	if got := problems(t, example); !strings.Contains(got, "does not register") {
		t.Errorf("an unregistered route must fail, got %q", got)
	}

	example.Uses.REST = []string{"/api/v1/status"}
	if got := problems(t, example); !strings.Contains(got, `not "METHOD /path"`) {
		t.Errorf("a route without a method must fail, got %q", got)
	}
}

func TestJ15ADeclarationMustMatchTheStepsBesideIt(t *testing.T) {
	// Declaring something the steps never run is drift in the other direction:
	// the reader is told this example shows a thing it does not show.
	example := recipe("curl -s http://127.0.0.1:8080/api/v1/status | jq")
	example.Uses.REST = []string{"POST /api/v1/documents"}
	if got := problems(t, example); !strings.Contains(got, "never calls it") {
		t.Errorf("a declared route the steps do not call must fail, got %q", got)
	}

	example = recipe("notriosctl paths --json")
	example.Uses.CLI = []string{"paths"}
	if got := problems(t, example); got != "" {
		t.Errorf("a command the spec has and the recipe runs: %s", got)
	}
	example.Uses.CLI = []string{"notes create"}
	if got := problems(t, example); !strings.Contains(got, "never runs it") {
		t.Errorf("a declared command the steps do not run must fail, got %q", got)
	}
	example.Uses.CLI = []string{"no-such-command"}
	if got := problems(t, example); !strings.Contains(got, "not in the CLI spec") {
		t.Errorf("a command the spec does not have must fail, got %q", got)
	}
}

func TestJ15AMethodAndPlaceholdersAreRead(t *testing.T) {
	// A published example puts a value where the route has a {placeholder},
	// and may splice a shell variable into the path.
	example := recipe(`curl -s -X DELETE http://127.0.0.1:8080/api/v1/documents/$DOC`)
	example.Uses.REST = []string{"DELETE /api/v1/documents/{document_id}"}
	if got := problems(t, example); got != "" {
		t.Errorf("a placeholder filled by a variable: %s", got)
	}
	// The method is part of the route: the same path with another verb is not
	// the same promise.
	example.Uses.REST = []string{"PUT /api/v1/documents/{document_id}"}
	if got := problems(t, example); !strings.Contains(got, "never calls it") {
		t.Errorf("a different method must fail, got %q", got)
	}
}

func TestJ15ASurfacesComeFromTheProgram(t *testing.T) {
	known := j15Surfaces(t)
	if len(known.commands) < 50 || len(known.routes) < 50 {
		t.Fatalf("%d commands and %d routes; this check would assert little",
			len(known.commands), len(known.routes))
	}
	if !known.commands["paths"] || !known.registers("GET /api/v1/status") {
		t.Error("the surfaces do not contain what the program plainly has")
	}
}
