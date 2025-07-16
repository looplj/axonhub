package xerrors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type tErr struct {
	msg string
}

func (t tErr) Error() string {
	return t.msg
}

type sErr struct {
	age  int
	name string
}

func (s sErr) Error() string {
	return fmt.Sprintf("age %d, name %s", s.age, s.name)
}

func (s *sErr) As(err interface{}) bool {
	switch e := err.(type) {
	case *tErr:
		e.msg = s.Error()
		return true
	default:
		return false
	}
}

func TestAs(t *testing.T) {
	type args struct {
		rawErr error
	}

	type caseT struct {
		name  string
		args  args
		want  tErr
		want1 bool
	}

	tests := []caseT{
		{
			name: "same type",
			args: args{
				rawErr: tErr{msg: "test"},
			},
			want:  tErr{msg: "test"},
			want1: true,
		},
		{
			name: "different type",
			args: args{
				rawErr: &sErr{
					age:  18,
					name: "test",
				},
			},
			want:  tErr{msg: "age 18, name test"},
			want1: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := As[tErr](tt.args.rawErr)
			assert.Equalf(t, tt.want, got, "As(%v)", tt.args.rawErr)
			assert.Equalf(t, tt.want1, got1, "As(%v)", tt.args.rawErr)
		})
	}
}
