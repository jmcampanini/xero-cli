package xero

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

func retryDelay(headers http.Header) time.Duration {
	seconds, err := strconv.Atoi(headers.Get("Retry-After"))
	if err != nil || seconds < 0 {
		seconds = 1
	}
	if seconds > 60 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

func responseError(status int, headers http.Header, body []byte) error {
	var problem struct {
		Detail   string
		Elements []struct{ ValidationErrors []struct{ Message string } }
		Message  string
		Title    string
		Type     string
	}
	if err := json.Unmarshal(body, &problem); err != nil {
		// The status still decides the code; the first 200 bytes become the message.
		if len(body) > 200 {
			body = body[:200]
		}
		problem.Message = string(body)
	}
	message := cmp.Or(problem.Message, problem.Detail, problem.Title, http.StatusText(status))
	code := "api"
	switch status {
	case 400:
		if problem.Type == "ValidationException" {
			code = "invalid_argument"
			var validation []string
			for _, element := range problem.Elements {
				for _, issue := range element.ValidationErrors {
					validation = append(validation, issue.Message)
				}
			}
			if len(validation) > 0 {
				message = strings.Join(validation, "; ")
			}
		}
	case 401:
		code = "unauthenticated"
	case 403:
		code = "forbidden"
		if problem.Detail != "" {
			message = problem.Detail
		}
	case 404:
		code = "not_found"
	case 429:
		code = "rate_limited"
		message = fmt.Sprintf("retry after %s", retryDelay(headers))
	}
	return apperr.New(code, "Xero HTTP %d: %s", status, strings.Join(strings.Fields(message), " "))
}
