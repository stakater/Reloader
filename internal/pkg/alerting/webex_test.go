package alerting

import "testing"

func TestEscapeWebexMarkdown(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain text is unchanged", in: "Triggered by Reloader in staging", want: "Triggered by Reloader in staging"},
		{name: "dashes, dots and digits are unchanged", in: "prod-eu 1.2", want: "prod-eu 1.2"},
		{name: "underscores", in: "env_prod_eu", want: `env\_prod\_eu`},
		{name: "asterisks", in: "*bold* and **strong**", want: `\*bold\* and \*\*strong\*\*`},
		{name: "leading heading marker", in: "# Cluster A", want: `\# Cluster A`},
		{name: "backticks", in: "run `kubectl get pods`", want: "run \\`kubectl get pods\\`"},
		{name: "link syntax", in: "[docs](https://example.com)", want: `\[docs\](https://example.com)`},
		{name: "quote marker", in: "> quoted", want: `\> quoted`},
		{name: "mention syntax", in: "<@all> look", want: `\<@all\> look`},
		{name: "backslash is escaped first", in: `a\_b`, want: `a\\\_b`},
		{name: "every line of a multi-line value", in: "line one\n# line two", want: "line one\n\\# line two"},
		{name: "empty", in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeWebexMarkdown(tt.in); got != tt.want {
				t.Errorf("escapeWebexMarkdown(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestWebexAlerter_FormatMessage_EscapesAdditionalInfo(t *testing.T) {
	a := NewWebexAlerter("http://example.com/webhook", "", "# env_prod_eu")

	got := a.formatMessage(testAlertMessage())

	wantPrefix := "\\# env\\_prod\\_eu\n\n**Reloader triggered reload**"
	if len(got) < len(wantPrefix) || got[:len(wantPrefix)] != wantPrefix {
		t.Errorf("formatMessage() = %q, want prefix %q", got, wantPrefix)
	}
}
