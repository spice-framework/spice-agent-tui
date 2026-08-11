# Code style

Application-owned Go in `internal/acceptance/composition` follows the
canonical Spice `java-structured` profile. Root [`CODE_STYLE.md`](../CODE_STYLE.md)
is a pointer to Spice commit
`0e79bc4f3b294cd0a429598c4921391f2e4d10e2`, whose reviewed file SHA-256 is
`09c014e2d7eb93bf2b395e24e4e6ff2466c05d164d4778a11cf7433164bffb76`.
This repository does not maintain a local reinterpretation of that contract.

The exact schema-2 policy is [`.spice/style.json`](../.spice/style.json). Its
source universe has two roots:

- `internal/acceptance/composition`, the only handwritten application source
  governed by this adoption; and
- `internal/spicegen`, solely so its real, nonempty
  `internal/spicegen/compositionproof` generated root can be declared and
  excluded from handwritten analysis.

Linux/amd64 and Windows/amd64 selections independently cover those exact roots.
Every applicable rule is enabled at error severity. Provider and application
function exceptions are limited to exact Spice contribution boundaries, and
the composition providers declare `@Singleton` explicitly.

`moduleOwnership` is the only inapplicable rule and is explicitly `off`.
Toolchain v0.1.0-preview.4 schema-2 roots forbid `.`, while this nested fixture
legitimately imports the module-root TUI package. Selecting only the nested
package cannot resolve the root package's `@Module` declaration; declaring it
as an allowed dependency would therefore fail as an unknown module. The
repository does not hide that limitation with an exemption or invented root.
Instead, the separate Spice composition gate always verifies the exact pair
`.` and `./internal/acceptance/composition`, which loads the root `@Module` and
authoritatively validates the fixture-to-root dependency. Mutations prove that
enabling `moduleOwnership` fails for this exact reason, that no second rule may
be disabled, and that the separate root-plus-fixture gate cannot be weakened.

The repository gate compares both policy files with their reviewed bytes,
requires every Go file under the generated root to carry the standard generated
marker, rejects generated Go outside that root, and rejects any generated marker
inside the handwritten composition root. This prevents a handwritten file from
escaping analysis by looking generated. The pinned Toolchain preview4
`spicestyle` then runs offline in `make check` and `make verify`.

Public runtime values, `terminal`, private presentation, `tuittest`,
auto-configuration, generated files, vendor, and the nested semantic-shell
experiment are intentionally outside this application-style boundary. Their
existing API, generated-ownership, historical-evidence, and dependency
contracts remain authoritative.
