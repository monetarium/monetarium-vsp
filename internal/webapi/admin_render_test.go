package webapi

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"regexp"

	"github.com/monetarium/monetarium-vsp/database"
	"github.com/monetarium/monetarium-vsp/internal/config"
	"strings"
	"testing"
)

// TestAdminTemplatesNesting renders the admin page (with and without a ticket
// search result) and checks that every <div> and <table> closes in the right
// order. Both pages are behind an admin login, so a broken layout there is
// easy to ship unnoticed; this catches it at build time instead.
func TestAdminTemplatesNesting(t *testing.T) {
	tmpl := template.New("").Funcs(template.FuncMap{
		"txURL": func(string) string { return "#" }, "addressURL": func(string) string { return "#" },
		"blockURL": func(int64) string { return "#" }, "dateTime": dateTime,
		"timeAgo": timeAgo, "stripWss": func(s string) string { return s },
		"indentJSON": func(string) string { return "" },
		"atomsToDCR": atomsToDCR, "comma": func(int64) string { return "" },
		"float32ToPercent": float32ToPercent,
		"pluralize":        pluralize,
		"supportURL":       supportURL, "supportLabel": supportLabel,
	})

	tmpl, err := tmpl.ParseGlob("templates/*.html")
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	data := map[string]any{
		"WebApiCache":   cacheData{},
		"WebApiCfg":     Config{Network: &config.TestNet3},
		"WalletStatus":  map[string]walletStatus{"127.0.0.1:19510": {}},
		"MondStatus":    mondStatus{},
		"MissedTickets": database.TicketList{{}},
		"CurrentXPub":   &database.FeeXPub{},
		"OldXPubs":      map[uint32]database.FeeXPub{1: {}},
		"SearchResult": &searchResult{
			Ticket:      database.Ticket{},
			VoteChanges: map[uint32]database.VoteChangeRecord{1: {}},
		},
	}

	for _, name := range []string{"admin.html", "homepage.html", "login.html"} {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			t.Fatalf("%s: execute: %v", name, err)
		}
		if err := checkNesting(buf.String()); err != nil {
			t.Errorf("%s: %v", name, err)
			os.WriteFile("/tmp/"+name+".render", buf.Bytes(), 0o600)
		}
	}
}

var tagRE = regexp.MustCompile(`<(/?)(div|table)\b[^>]*>`)

func checkNesting(html string) error {
	var stack []string
	for _, m := range tagRE.FindAllStringSubmatch(html, -1) {
		closing, tag := m[1], m[2]
		if closing == "" {
			stack = append(stack, tag)
			continue
		}
		if len(stack) == 0 {
			return errFmt("closing </%s> with nothing open", tag)
		}
		if top := stack[len(stack)-1]; top != tag {
			return errFmt("closing </%s> while <%s> is open", tag, top)
		}
		stack = stack[:len(stack)-1]
	}
	if len(stack) > 0 {
		return errFmt("unclosed: %s", strings.Join(stack, ", "))
	}
	return nil
}

func errFmt(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
