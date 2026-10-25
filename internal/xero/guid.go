package xero

import (
	"regexp"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

var guidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// RequireGUID rejects values that are not GUIDs.
func RequireGUID(value string) error {
	if !guidPattern.MatchString(value) {
		return apperr.New("invalid_argument", "%q must be a GUID", value)
	}
	return nil
}
