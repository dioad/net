package net

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isTLSCloseNotifyError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "tls close_notify with connection closed anyway",
			err:  errors.New("tls: failed to send closeNotify alert (but connection was closed anyway): write tcp 10.0.0.1:443->1.2.3.4:5678: write: broken pipe"),
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "net.ErrClosed",
			err:  net.ErrClosed,
			want: false,
		},
		{
			name: "generic broken pipe",
			err:  errors.New("write: broken pipe"),
			want: false,
		},
		{
			name: "other tls error without closed-anyway phrase",
			err:  errors.New("tls: bad record MAC"),
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isTLSCloseNotifyError(tc.err))
		})
	}
}

func Test_IsBenignConnCloseError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "net.ErrClosed",
			err:  net.ErrClosed,
			want: true,
		},
		{
			name: "tls close_notify with connection closed anyway",
			err:  errors.New("tls: failed to send closeNotify alert (but connection was closed anyway): write tcp 10.0.0.1:443->1.2.3.4:5678: write: broken pipe"),
			want: true,
		},
		{
			name: "generic tls error",
			err:  errors.New("tls: bad record MAC"),
			want: false,
		},
		{
			name: "generic broken pipe",
			err:  errors.New("write: broken pipe"),
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, IsBenignConnCloseError(tc.err))
		})
	}
}
