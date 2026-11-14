package httpapi

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

var componentName = regexp.MustCompile(`^[A-Z][A-Za-z0-9]{1,49}$`)

type scaffoldSpec struct {
	Title       string
	Description string
	Tags        []string
	Acceptance  []domain.AcceptanceCriterion
}

// componentScaffold builds the task spec for a new UI component.
func componentScaffold(name, kind, designStandardID string) (scaffoldSpec, error) {
	name = strings.TrimSpace(name)
	kind = strings.ToLower(strings.TrimSpace(kind))
	if !componentName.MatchString(name) {
		return scaffoldSpec{}, fmt.Errorf("component name must be PascalCase (2-50 characters)")
	}
	var files []string
	switch kind {
	case "react":
		files = []string{
			"packages/ui/src/components/" + name + "/" + name + ".tsx",
			"packages/ui/src/components/" + name + "/" + name + ".test.tsx",
			"packages/ui/src/components/" + name + "/" + name + ".stories.tsx",
		}
	case "vue":
		files = []string{
			"packages/ui/src/components/" + name + "/" + name + ".vue",
			"packages/ui/src/components/" + name + "/" + name + ".test.ts",
			"packages/ui/src/components/" + name + "/" + name + ".stories.ts",
		}
	case "angular":
		files = []string{
			"packages/ui/src/components/" + name + "/" + name + ".component.ts",
			"packages/ui/src/components/" + name + "/" + name + ".component.spec.ts",
			"packages/ui/src/components/" + name + "/" + name + ".stories.ts",
		}
	default:
		return scaffoldSpec{}, fmt.Errorf("kind must be react, vue or angular")
	}
	design := designStandardID
	if design == "" {
		design = "(project default)"
	}
	var b strings.Builder
	b.WriteString("Create the UI component " + name + " (" + kind + ").\n\n")
	b.WriteString("Design standard: " + design + "\n")
	b.WriteString("Requirements:\n")
	b.WriteString("- use design tokens only (no hardcoded colours/spacing)\n")
	b.WriteString("- expose the component in the component registry with variants/props\n")
	b.WriteString("- add unit tests covering the rendered states\n")
	b.WriteString("- add a Storybook story and a visual fixture under visual/*.png if the project uses visual checks\n\n")
	b.WriteString("Expected files:\n")
	for _, file := range files {
		b.WriteString("- " + file + "\n")
	}
	return scaffoldSpec{
		Title:       "Scaffold " + name + " component",
		Description: b.String(),
		Tags:        []string{"component", kind, "scaffold"},
		Acceptance: []domain.AcceptanceCriterion{{
			Given: "the component " + name + " is implemented",
			When:  "the design check and unit tests run",
			Then:  "the component uses tokens, is registered and its tests pass",
		}},
	}, nil
}
