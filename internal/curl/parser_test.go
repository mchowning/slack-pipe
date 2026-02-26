package curl_test

import (
	"testing"

	"github.com/mchowning/slack-pipe/internal/curl"
)

// Sample cURL commands from real Slack API request formats (tokens anonymized).
const sampleCurl = `curl 'https://myworkspace.slack.com/api/conversations.view?_x_id=noversion-1770041775.173' \
  -H 'accept: */*' \
  -b 'cjConsent=MHxZfDB8Tnww; d=xoxd-XXXXXXXX-XXXXXXXX-XXXXXXXXXXXXXXXX; lc=1770041685' \
  --data-raw $'------WebKitFormBoundaryBPkbAXdra05yI37u\r\nContent-Disposition: form-data; name="token"\r\n\r\nxoxc-XXXXXXXXXX-XXXXXXXXXX-XXXXXXXXXXXXXXXXXX\r\n------WebKitFormBoundaryBPkbAXdra05yI37u--\r\n'`

const curlWithCookieHeader = `curl 'https://testteam.slack.com/api/users.list' \
  -H 'Cookie: d=xoxd-AAAABBBBCCCCDDDD-1234567890' \
  --data-raw $'------WebKitFormBoundary\r\nContent-Disposition: form-data; name="token"\r\n\r\nxoxc-111222333-444555666-abcdefghij\r\n------WebKitFormBoundary--\r\n'`

const curlWithCookieFlag = `curl 'https://anotherteam.slack.com/api/channels.list' \
  --cookie 'd=xoxd-TESTTOKEN-12345; other=value' \
  --data $'------Boundary\r\nContent-Disposition: form-data; name="token"\r\n\r\nxoxc-test-token-123\r\n------Boundary--\r\n'`

const curlURLEncoded = `curl 'https://encoded.slack.com/api/test' \
  -b 'd=xoxd-encoded%2Btoken%2Fwith%3Dspecial' \
  --data-raw $'------Boundary\r\nContent-Disposition: form-data; name="token"\r\n\r\nxoxc-encoded-test\r\n------Boundary--\r\n'`

const curlSingleLine = `curl 'https://singleline.slack.com/api/test' -b 'd=xoxd-single-line-token' --data-raw $'------Boundary\r\nContent-Disposition: form-data; name="token"\r\n\r\nxoxc-single-line\r\n------Boundary--\r\n'`

func TestParse(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantWorkspace string
		wantURL       string
		wantXoxc      string
		wantCookie    string
	}{
		{
			name:          "standard curl with -b flag",
			input:         sampleCurl,
			wantWorkspace: "myworkspace",
			wantURL:       "https://myworkspace.slack.com",
			wantXoxc:      "xoxc-XXXXXXXXXX-XXXXXXXXXX-XXXXXXXXXXXXXXXXXX",
			wantCookie:    "xoxd-XXXXXXXX-XXXXXXXX-XXXXXXXXXXXXXXXX",
		},
		{
			name:          "curl with -H Cookie header",
			input:         curlWithCookieHeader,
			wantWorkspace: "testteam",
			wantURL:       "https://testteam.slack.com",
			wantXoxc:      "xoxc-111222333-444555666-abcdefghij",
			wantCookie:    "xoxd-AAAABBBBCCCCDDDD-1234567890",
		},
		{
			name:          "curl with --cookie flag",
			input:         curlWithCookieFlag,
			wantWorkspace: "anotherteam",
			wantURL:       "https://anotherteam.slack.com",
			wantXoxc:      "xoxc-test-token-123",
			wantCookie:    "xoxd-TESTTOKEN-12345",
		},
		{
			name:          "URL-encoded cookie is decoded",
			input:         curlURLEncoded,
			wantWorkspace: "encoded",
			wantURL:       "https://encoded.slack.com",
			wantXoxc:      "xoxc-encoded-test",
			wantCookie:    "xoxd-encoded+token/with=special",
		},
		{
			name:          "single-line curl",
			input:         curlSingleLine,
			wantWorkspace: "singleline",
			wantURL:       "https://singleline.slack.com",
			wantXoxc:      "xoxc-single-line",
			wantCookie:    "xoxd-single-line-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := curl.Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.WorkspaceName != tt.wantWorkspace {
				t.Errorf("workspace: got %q, want %q", result.WorkspaceName, tt.wantWorkspace)
			}
			if result.WorkspaceURL != tt.wantURL {
				t.Errorf("URL: got %q, want %q", result.WorkspaceURL, tt.wantURL)
			}
			if result.XoxcToken != tt.wantXoxc {
				t.Errorf("xoxc: got %q, want %q", result.XoxcToken, tt.wantXoxc)
			}
			if result.DCookie != tt.wantCookie {
				t.Errorf("cookie: got %q, want %q", result.DCookie, tt.wantCookie)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantField string
	}{
		{
			name:      "missing workspace URL",
			input:     `curl 'https://example.com/api/test' -b 'd=xoxd-token'`,
			wantField: "workspace",
		},
		{
			name:      "missing xoxd cookie",
			input:     `curl 'https://test.slack.com/api/test' -b 'other=value' --data-raw $'------Boundary\r\nContent-Disposition: form-data; name="token"\r\n\r\nxoxc-test\r\n------Boundary--\r\n'`,
			wantField: "cookie",
		},
		{
			name:      "missing xoxc token",
			input:     `curl 'https://test.slack.com/api/test' -b 'd=xoxd-test' --data-raw $'------Boundary\r\nContent-Disposition: form-data; name="other"\r\n\r\nvalue\r\n------Boundary--\r\n'`,
			wantField: "xoxc",
		},
		{
			name:      "missing data entirely",
			input:     `curl 'https://test.slack.com/api/test' -b 'd=xoxd-test'`,
			wantField: "xoxc",
		},
		{
			name:      "missing cookie header entirely",
			input:     `curl 'https://test.slack.com/api/test' --data-raw $'------Boundary\r\nContent-Disposition: form-data; name="token"\r\n\r\nxoxc-test\r\n------Boundary--\r\n'`,
			wantField: "cookie",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := curl.Parse(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			pe, ok := err.(*curl.ParseError)
			if !ok {
				t.Fatalf("expected *ParseError, got %T", err)
			}
			if pe.Field != tt.wantField {
				t.Errorf("field: got %q, want %q", pe.Field, tt.wantField)
			}
		})
	}
}

func TestLooksLikeCurl(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"curl 'https://example.com'", true},
		{"  curl https://example.com", true},
		{"curl\thttps://example.com", true},
		{"\n curl https://example.com", true},
		{"https://example.com", false},
		{"wget https://example.com", false},
		{"", false},
		{"curlhttps://example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := curl.LooksLikeCurl(tt.input); got != tt.want {
				t.Errorf("LooksLikeCurl(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseErrorString(t *testing.T) {
	pe := &curl.ParseError{Field: "cookie", Message: "test error"}
	s := pe.Error()
	if s != "parse error (cookie): test error" {
		t.Fatalf("unexpected error string: %s", s)
	}
}

func TestValidateTokenFormat(t *testing.T) {
	tests := []struct {
		token  string
		prefix string
		ok     bool
	}{
		{"xoxc-abc-123", "xoxc-", true},
		{"xoxd-abc-123", "xoxd-", true},
		{"xoxb-abc-123", "xoxc-", false},
		{"", "xoxc-", false},
		{"short", "xoxc-", false},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			err := curl.ValidateTokenFormat(tt.token, tt.prefix)
			if tt.ok && err != nil {
				t.Errorf("expected ok, got %v", err)
			}
			if !tt.ok && err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}
