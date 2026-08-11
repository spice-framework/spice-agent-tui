package composition

import agenttui "github.com/spice-framework/spice-agent-tui"

// @import { Bean, Singleton } from "github.com/spice-framework/spice/annotation/core"

// NewSession is the application-owned exact session bean required to activate
// the terminal-shell fallback.
//
// @Bean(name="acceptanceSession")
// @Singleton
func NewSession() agenttui.Session { return session{} }
