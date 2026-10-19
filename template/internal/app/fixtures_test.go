package app

import (
	"bytes"
	"flag"
	"testing"

	"github.com/flare-foundation/fce-go-scaffold/conformance"
	"github.com/flare-foundation/fce-go-scaffold/internal/golden"
	"github.com/flare-foundation/tee-node/pkg/types"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the conformance fixtures")

// fixturesDir is the extension's conformance directory, relative to this package.
const fixturesDir = "../../conformance"

// fixtures are the golden cases `fce conformance` checks the binary against.
func fixtures(t *testing.T) []conformance.Fixture {
	t.Helper()
	hello := encodeMyCommand(t, []byte("hello"))
	helloJSON := []byte(`{"message":"hello"}`)
	return []conformance.Fixture{
		{
			Name: "my-command", Description: "echoes the sender and the message, ABI-encoded",
			Channel: types.Instruction, OPType: OPTypeMyExtension, OPCommand: OPCommandMyCommand,
			Message: hello, Result: conformance.Expected{Status: 1, Log: "ok", Data: hello},
		},
		{
			Name: "my-command-empty-message", Description: "refuses an empty message",
			Channel: types.Instruction, OPType: OPTypeMyExtension, OPCommand: OPCommandMyCommand,
			Message: encodeMyCommand(t, nil), Result: conformance.Expected{Status: 0, Log: "error: message required"},
		},
		{
			Name: "my-command-not-canonical", Description: "refuses a payload with a trailing byte",
			Channel: types.Instruction, OPType: OPTypeMyExtension, OPCommand: OPCommandMyCommand,
			Message: append(bytes.Clone(hello), 0), Result: conformance.Expected{Status: 0, Log: "error: invalid payload"},
		},
		{
			Name: "my-direct-command", Description: "echoes the message as JSON",
			Channel: types.Direct, OPType: OPTypeMyExtension, OPCommand: OPCommandMyDirectCommand,
			Message: helloJSON, Result: conformance.Expected{Status: 1, Log: "ok", Data: helloJSON},
		},
		{
			Name: "my-direct-command-empty-message", Description: "refuses an empty message",
			Channel: types.Direct, OPType: OPTypeMyExtension, OPCommand: OPCommandMyDirectCommand,
			Message: []byte(`{"message":""}`), Result: conformance.Expected{Status: 0, Log: "error: message required"},
		},
		{
			Name: "my-direct-command-unknown-field", Description: "refuses a field the command doesn't define",
			Channel: types.Direct, OPType: OPTypeMyExtension, OPCommand: OPCommandMyDirectCommand,
			Message: []byte(`{"message":"hello","extra":1}`), Result: conformance.Expected{Status: 0, Log: "error: invalid payload"},
		},
	}
}

// TestFixtures keeps the committed fixtures in step with fixtures:
// `go test ./template/internal/app -update` rewrites them.
func TestFixtures(t *testing.T) {
	t.Parallel()
	require.NoError(t, golden.Check(fixturesDir, fixtures(t), *update))
}
