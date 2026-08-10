// Package semanticshell provides an experimental standard-library shell over
// the public Spice Agent TUI Session contract.
//
// It deliberately owns no transport, daemon, plugin, terminal emulator, retry
// policy, or executable UI extension. Input is a bounded line command stream;
// output is a deterministic, bounded JSON Lines semantic stream.
package semanticshell
