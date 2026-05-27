package runtime

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	unableToFindImagePattern = regexp.MustCompile(`(?i)unable to find image ['"]?([^'"\s]+)['"]? locally`)
	registryLookupPattern    = regexp.MustCompile(`(?i)\blookup\s+([a-z0-9.-]+\.[a-z]{2,})\b[^:]*:\s*(temporary failure in name resolution|no such host|server misbehaving)`)
)

type commandFailure struct {
	detail        string
	publicMessage string
	cause         error
}

func (e *commandFailure) Error() string {
	return e.detail
}

func (e *commandFailure) Unwrap() error {
	return e.cause
}

func (e *commandFailure) PublicMessage() string {
	return e.publicMessage
}

func PublicErrorMessage(err error) (string, bool) {
	var publicErr interface {
		error
		PublicMessage() string
	}
	if !errors.As(err, &publicErr) {
		return "", false
	}
	message := strings.TrimSpace(publicErr.PublicMessage())
	if message == "" {
		return "", false
	}
	return message, true
}

func newCommandFailure(binary string, args []string, result commandResult, err error, publicMessage string) error {
	return &commandFailure{
		detail:        formatCommandFailure(binary, args, result, err),
		publicMessage: strings.TrimSpace(publicMessage),
		cause:         err,
	}
}

func formatCommandFailure(binary string, args []string, result commandResult, err error) string {
	detail := strings.TrimSpace(result.stderr)
	if detail == "" {
		detail = strings.TrimSpace(result.stdout)
	}
	if detail == "" {
		return fmt.Sprintf("%s %s: %v", binary, strings.Join(args, " "), err)
	}
	return fmt.Sprintf("%s %s: %v: %s", binary, strings.Join(args, " "), err, detail)
}

func classifyCommandPublicMessage(image string, result commandResult) string {
	detail := strings.TrimSpace(result.stderr)
	if detail == "" {
		detail = strings.TrimSpace(result.stdout)
	}
	if detail == "" {
		return ""
	}
	if message := classifyRegistryNetworkMessage(detail); message != "" {
		return message
	}
	return classifyImageNotFoundMessage(image, detail)
}

func classifyRegistryNetworkMessage(detail string) string {
	matches := registryLookupPattern.FindStringSubmatch(detail)
	if len(matches) < 2 {
		return ""
	}
	registry := strings.TrimSpace(matches[1])
	if registry == "" {
		return "container registry DNS lookup failed; check Docker/Podman VM DNS settings"
	}
	return fmt.Sprintf("container registry %q DNS lookup failed; check Docker/Podman VM DNS settings", registry)
}

func classifyImageNotFoundMessage(image, detail string) string {
	if !isImageNotFoundDetail(detail) {
		return ""
	}

	image = strings.TrimSpace(image)
	if image == "" {
		matches := unableToFindImagePattern.FindStringSubmatch(detail)
		if len(matches) > 1 {
			image = strings.TrimSpace(matches[1])
		}
	}
	if image == "" {
		return "container image not found"
	}
	return fmt.Sprintf("image %q not found", image)
}

func isImageNotFoundDetail(detail string) bool {
	lowerDetail := strings.ToLower(detail)
	return strings.Contains(lowerDetail, "unable to find image") ||
		strings.Contains(lowerDetail, "pull access denied") ||
		strings.Contains(lowerDetail, "repository does not exist") ||
		strings.Contains(lowerDetail, "manifest unknown") ||
		strings.Contains(lowerDetail, "no such image") ||
		strings.Contains(lowerDetail, "image not known")
}
