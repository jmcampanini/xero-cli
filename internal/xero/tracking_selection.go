package xero

import (
	"strings"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

// ResolveTrackingSelection resolves a CATEGORY=OPTION filter by exact names ignoring case.
func ResolveTrackingSelection(categories []TrackingCategory, filter string) (TrackingCategory, TrackingOption, error) {
	name, optionName, _ := strings.Cut(filter, "=")
	category, err := ResolveTrackingCategory(categories, name)
	if err != nil {
		return TrackingCategory{}, TrackingOption{}, err
	}
	var matches []TrackingOption
	var names []string
	for _, option := range category.Options {
		names = append(names, option.Name)
		if strings.EqualFold(option.Name, optionName) {
			matches = append(matches, option)
		}
	}
	if len(matches) != 1 {
		return TrackingCategory{}, TrackingOption{}, apperr.New("invalid_argument", "option %q in %q must match exactly one option; %s", optionName, category.Name, available("options", names))
	}
	return category, matches[0], nil
}

// ResolveTrackingCategory requires exactly one category matching the name ignoring case.
func ResolveTrackingCategory(categories []TrackingCategory, name string) (TrackingCategory, error) {
	var matches []TrackingCategory
	var names []string
	for _, category := range categories {
		names = append(names, category.Name)
		if strings.EqualFold(category.Name, name) {
			matches = append(matches, category)
		}
	}
	if len(matches) != 1 {
		return TrackingCategory{}, apperr.New("invalid_argument", "category %q must match exactly one category; %s", name, available("tracking categories", names))
	}
	return matches[0], nil
}

func available(noun string, names []string) string {
	if len(names) == 0 {
		return "no " + noun + " exist"
	}
	return "available: " + strings.Join(names, ", ")
}
