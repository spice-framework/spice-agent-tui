// Package anonymous is a negative fixture for anonymous interface results.
package anonymous

import "context"

// @import { UIShell } from "github.com/spice-framework/spice-agent-tui/annotation/ui"

// NewShell returns an anonymous interface that must not impersonate Shell.
// @UIShell(name="anonymous-shell")
func NewShell() interface{ Run(context.Context) error } { return nil }
