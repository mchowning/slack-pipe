package curl

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type ParsedResult struct {
	WorkspaceName string
	WorkspaceURL  string
	XoxcToken     string // xoxc-...
	DCookie       string // xoxd-... (the d cookie value, URL-decoded)
}

type ParseError struct {
	Field   string // "workspace", "xoxc", "cookie"
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error (%s): %s", e.Field, e.Message)
}

var (
	urlPattern    = regexp.MustCompile(`curl\s+'?(https?://([^.]+)\.slack\.com[^'"\s]*)`)
	cookieBFlag   = regexp.MustCompile(`-b\s+'([^']+)'`)
	cookieLong    = regexp.MustCompile(`--cookie\s+'([^']+)'`)
	cookieHeader  = regexp.MustCompile(`-H\s+'[Cc]ookie:\s*([^']+)'`)
	xoxdPattern   = regexp.MustCompile(`(?:^|;\s*)d=(xoxd-[^;]+)`)
	dataRawSingle = regexp.MustCompile(`--data-raw\s+\$?'([^']+)'`)
	dataRawDouble = regexp.MustCompile(`--data-raw\s+\$?"([^"]+)"`)
	dataSingle    = regexp.MustCompile(`--data\s+\$?'([^']+)'`)
	dataDouble    = regexp.MustCompile(`--data\s+\$?"([^"]+)"`)
	xoxcPattern   = regexp.MustCompile(`(?s)name="token".*?(xoxc-[a-zA-Z0-9-]+)`)
)

func Parse(input string) (*ParsedResult, error) {
	// Extract workspace URL
	urlMatch := urlPattern.FindStringSubmatch(input)
	if urlMatch == nil {
		return nil, &ParseError{Field: "workspace", Message: "could not find Slack workspace URL in cURL command"}
	}
	workspaceName := urlMatch[2]
	workspaceURL := fmt.Sprintf("https://%s.slack.com", workspaceName)

	// Extract d cookie from cookie header/flag
	cookieStr := extractCookie(input)
	if cookieStr == "" {
		return nil, &ParseError{Field: "cookie", Message: "could not find cookie header (-b, --cookie, or -H 'Cookie:')"}
	}
	xoxdMatch := xoxdPattern.FindStringSubmatch(cookieStr)
	if xoxdMatch == nil {
		return nil, &ParseError{Field: "cookie", Message: "could not find xoxd token in cookie (d=xoxd-...)"}
	}
	dCookie, err := url.QueryUnescape(xoxdMatch[1])
	if err != nil {
		dCookie = xoxdMatch[1] // Fall back to raw value
	}

	// Extract xoxc token from request data
	dataStr := extractData(input)
	if dataStr == "" {
		return nil, &ParseError{Field: "xoxc", Message: "could not find request data (--data-raw or --data)"}
	}
	xoxcMatch := xoxcPattern.FindStringSubmatch(dataStr)
	if xoxcMatch == nil {
		return nil, &ParseError{Field: "xoxc", Message: "could not find xoxc token in request data"}
	}

	return &ParsedResult{
		WorkspaceName: workspaceName,
		WorkspaceURL:  workspaceURL,
		XoxcToken:     xoxcMatch[1],
		DCookie:       dCookie,
	}, nil
}

func extractCookie(input string) string {
	for _, re := range []*regexp.Regexp{cookieBFlag, cookieLong, cookieHeader} {
		if m := re.FindStringSubmatch(input); m != nil {
			return m[1]
		}
	}
	return ""
}

func extractData(input string) string {
	for _, re := range []*regexp.Regexp{dataRawSingle, dataRawDouble, dataSingle, dataDouble} {
		if m := re.FindStringSubmatch(input); m != nil {
			return m[1]
		}
	}
	return ""
}

func LooksLikeCurl(input string) bool {
	trimmed := strings.TrimSpace(input)
	return strings.HasPrefix(trimmed, "curl ") || strings.HasPrefix(trimmed, "curl\t")
}

// ValidateTokenFormat checks that a token has the expected prefix.
func ValidateTokenFormat(token, expectedPrefix string) error {
	if !strings.HasPrefix(token, expectedPrefix) {
		return fmt.Errorf("token must start with %s, got: %s", expectedPrefix, safePrefix(token, 10))
	}
	return nil
}

func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
