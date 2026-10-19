package app

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/fce-go-scaffold/pkg/payload"
	"github.com/flare-foundation/go-flare-common/pkg/tee/instruction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSender = common.HexToAddress("0x00000000000000000000000000000000000000c0")

// encodeMyCommand builds the ABI payload as the contract's abi.encode(msg.sender, message) does.
func encodeMyCommand(t *testing.T, message []byte) []byte {
	t.Helper()
	b, err := myCommandArgs.Pack(testSender, message)
	require.NoError(t, err)
	return b
}

func TestMyCommand(t *testing.T) {
	t.Parallel()
	for _, message := range [][]byte{[]byte("hello"), bytes.Repeat([]byte{0xff}, 33)} {
		in := encodeMyCommand(t, message)
		out, run, err := myCommand(t.Context(), &instruction.DataFixed{OriginalMessage: in})
		require.NoError(t, err)
		assert.Nil(t, run)

		var p myCommandPayload
		require.NoError(t, payload.DecodeABI(myCommandArgs, out, &p))
		assert.Equal(t, testSender, p.Sender)
		assert.Equal(t, message, p.Message)
	}
}

func TestMyCommandRefuses(t *testing.T) {
	t.Parallel()
	good := encodeMyCommand(t, []byte("hello"))
	dirty := bytes.Clone(good)
	dirty[0] = 0xff // address padding

	tests := []struct {
		name, in string
		want     string
	}{
		{"empty", "", "invalid payload"},
		{"truncated", string(good[:len(good)-32]), "invalid payload"},
		{"trailing byte", string(append(bytes.Clone(good), 0)), "invalid payload"},
		{"dirty padding", string(dirty), "invalid payload"},
		{"empty message", string(encodeMyCommand(t, nil)), "message required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, _, err := myCommand(t.Context(), &instruction.DataFixed{OriginalMessage: []byte(tt.in)})
			require.Error(t, err)
			assert.Equal(t, tt.want, err.Error())
			assert.Nil(t, out)
		})
	}
}
