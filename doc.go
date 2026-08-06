// Package agenttui defines bounded, immutable, UI-neutral terminal contracts
// for the standalone Spice Agent terminal product.
//
// The package owns semantic view data, prompt editing, key bindings, themes,
// deterministic rendering, and shell lifecycle contracts. It deliberately does
// not define or discover a daemon, transport, coding-agent client, annotation
// model, generated application, or executable entrypoint. Those boundaries must
// be explicitly injected after their owning contracts are adopted.
package agenttui
