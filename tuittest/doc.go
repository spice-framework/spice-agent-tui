// Package tuittest provides a deterministic, agent-friendly harness for the
// Spice Agent TUI.
//
// It is designed so coding agents and humans can:
//
//   - drive keyboard interaction without a real PTY;
//   - inject Session updates without a daemon;
//   - capture exact terminal frames (styled and plain);
//   - assert pixel-perfect goldens with optional UPDATE_GOLDEN refresh;
//   - dump multi-format screen reports that agents can read in logs.
//
// The harness uses the real presentation Model and FixedRenderer. It does not
// start Bubble Tea's event loop, discover a daemon, or perform network I/O.
//
// Typical agent workflow:
//
//	driver, err := tuittest.NewDriver(tuittest.Options{Width: 48, Height: 12})
//	// ...
//	defer driver.Close()
//	_ = driver.Type("list owners")
//	_ = driver.Key("enter")
//	screen, _ := driver.Screen("list-owners")
//	fmt.Print(screen.AgentReport())
//	screen.AssertGolden(t, "testdata", "list-owners")
package tuittest
