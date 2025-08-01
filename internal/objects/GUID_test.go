package objects

import (
	"bytes"
	"testing"
)

func TestGUID_MarshalGQL(t *testing.T) {
	type fields struct {
		Type string
		UUID int
	}
	tests := []struct {
		name   string
		fields fields
		wantW  string
	}{
		{
			name: "gid",
			fields: fields{
				Type: "type",
				UUID: 1,
			},
			wantW: `"gid://type/1"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guid := GUID{
				Type: tt.fields.Type,
				ID:   tt.fields.UUID,
			}
			w := &bytes.Buffer{}
			guid.MarshalGQL(w)
			if gotW := w.String(); gotW != tt.wantW {
				t.Errorf("GUID.MarshalGQL() = %v, want %v", gotW, tt.wantW)
			}
		})
	}
}

func TestGUID_UnmarshalGQL(t *testing.T) {
	type fields struct {
		Type string
		ID   int
	}
	type args struct {
		v any
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "gid",
			fields: fields{
				Type: "type",
				ID:   1,
			},
			args: args{
				v: "gid://type/1",
			},
		},
		{
			name: "empty",
			fields: fields{
				Type: "",
				ID:   0,
			},
			args: args{
				v: "",
			},
			wantErr: true,
		},
		{
			name: "invalid",
			fields: fields{
				Type: "type",
				ID:   0,
			},
			args: args{
				v: "gid://type/invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid prefix",
			fields: fields{
				Type: "type",
				ID:   0,
			},
			args: args{
				v: "guid://invalid/1",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guid := &GUID{
				Type: tt.fields.Type,
				ID:   tt.fields.ID,
			}
			if err := guid.UnmarshalGQL(tt.args.v); (err != nil) != tt.wantErr {
				t.Errorf("GUID.UnmarshalGQL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
