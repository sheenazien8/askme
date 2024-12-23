package form

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
)

func BuildPromptForm(prompt *string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				CharLimit(0).
				Placeholder("Enter a prompt").
				Value(prompt).
				Validate(func(s string) error {

					s = strings.TrimSpace(s)
					if s == "" {
						return fmt.Errorf("A prompt is required")
					}
					return nil
				}),
		),
	)
}

