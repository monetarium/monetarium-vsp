// Copyright (c) 2020-2024 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package webapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/decred/slog"
	"github.com/dustin/go-humanize"
	"github.com/monetarium/monetarium-node/dcrutil"
)

// supportURL turns the configured support contact into something a browser can
// follow. Upstream assumed an email address and hardcoded a mailto: prefix in
// the template; Monetarium's published contact channels are a Telegram group
// and GitHub, so the value may just as well be a URL. An address gets mailto:,
// anything already carrying a scheme is used as-is.
func supportURL(contact string) string {
	switch {
	case strings.Contains(contact, "://"):
		return contact
	case strings.Contains(contact, "@"):
		return "mailto:" + contact
	default:
		return contact
	}
}

// supportLabel is what the support link reads as: a bare address or, for a URL,
// its host and path without the scheme — "t.me/monetarium_world" rather than
// the full "https://t.me/monetarium_world".
func supportLabel(contact string) string {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(contact, "https://"), "http://")
	return strings.TrimSuffix(trimmed, "/")
}

func addressURL(blockExplorerURL string) func(string) string {
	return func(addr string) string {
		return fmt.Sprintf("%s/address/%s", blockExplorerURL, addr)
	}
}

func txURL(blockExplorerURL string) func(string) string {
	return func(txID string) string {
		return fmt.Sprintf("%s/tx/%s", blockExplorerURL, txID)
	}
}

func blockURL(blockExplorerURL string) func(int64) string {
	return func(height int64) string {
		return fmt.Sprintf("%s/block/%d", blockExplorerURL, height)
	}
}

// dateTime returns a human readable representation of the provided unix
// timestamp. It includes the local timezone of the server so use on public
// webpages is not recommended.
func dateTime(t int64) string {
	return time.Unix(t, 0).Format("2 Jan 2006 15:04:05 MST")
}

// timeAgo compares the provided unix timestamp to the current time to return a
// string like "3 minutes ago".
func timeAgo(t time.Time) string {
	return humanize.Time(t)
}

func stripWss(input string) string {
	input = strings.ReplaceAll(input, "wss://", "")
	input = strings.ReplaceAll(input, "/ws", "")
	return input
}

// indentJSON returns a func which uses whitespace to format a provided JSON
// string. If the parameter is invalid JSON, an error will be logged and the
// param will be returned unaltered.
func indentJSON(log slog.Logger) func(string) string {
	return func(input string) string {
		var indented bytes.Buffer
		err := json.Indent(&indented, []byte(input), "", "    ")
		if err != nil {
			log.Errorf("Failed to indent JSON: %w", err)
			return input
		}

		return indented.String()
	}
}

func atomsToDCR(atoms int64) string {
	return dcrutil.Amount(atoms).String()
}

func float32ToPercent(input float32) string {
	return fmt.Sprintf("%.2f%%", input*100)
}

// pluralize suffixes the provided noun with "s" if n is not 1, then
// concatenates n and noun with a space between them. For example:
//
//	(0, "biscuit") will return "0 biscuits"
//	(1, "biscuit") will return "1 biscuit"
//	(3, "biscuit") will return "3 biscuits"
func pluralize(n int, noun string) string {
	if n != 1 {
		noun += "s"
	}
	return fmt.Sprintf("%d %s", n, noun)
}
