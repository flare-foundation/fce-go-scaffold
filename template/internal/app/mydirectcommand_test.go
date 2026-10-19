package app

import (
	"testing"

	"github.com/flare-foundation/tee-node/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMyDirectCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{`{"message":"hello"}`, `{"message":"hello"}`},
		{` {"message": "hello"} `, `{"message":"hello"}`},
	}
	for _, tt := range tests {
		out, run, err := myDirectCommand(t.Context(), &types.DirectInstruction{Message: []byte(tt.in)})
		require.NoError(t, err, tt.in)
		assert.Nil(t, run)
		assert.JSONEq(t, tt.want, string(out), tt.in)
	}
}

func TestMyDirectCommandRefuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in string
		want     string
	}{
		{"empty", ``, "invalid payload"},
		{"unknown field", `{"message":"hello","extra":1}`, "invalid payload"},
		{"trailing garbage", `{"message":"hello"}x`, "invalid payload"},
		{"empty message", `{"message":""}`, "message required"},
		{"no message", `{}`, "message required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, _, err := myDirectCommand(t.Context(), &types.DirectInstruction{Message: []byte(tt.in)})
			require.Error(t, err)
			assert.Equal(t, tt.want, err.Error())
			assert.Nil(t, out)
		})
	}
}
