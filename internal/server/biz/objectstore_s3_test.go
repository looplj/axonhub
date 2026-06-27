package biz

import (
	"errors"
	"fmt"
	"testing"

	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
)

func TestIsS3NotFound(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("boom"), false},
		{"NoSuchKey (GetObject)", &s3types.NoSuchKey{}, true},
		{"NotFound (HeadObject)", &s3types.NotFound{}, true},
		{"wrapped NoSuchKey", fmt.Errorf("get: %w", &s3types.NoSuchKey{}), true},
		{"smithy code NoSuchKey", &smithy.GenericAPIError{Code: "NoSuchKey"}, true},
		{"smithy code NotFound", &smithy.GenericAPIError{Code: "NotFound"}, true},
		{"smithy code 404", &smithy.GenericAPIError{Code: "404"}, true},
		{"smithy other code", &smithy.GenericAPIError{Code: "AccessDenied"}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isS3NotFound(c.err); got != c.want {
				t.Errorf("isS3NotFound(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

func TestNormalizeObjectKey(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/2/requests/5/request_body.json", "2/requests/5/request_body.json"},
		{"2/requests/5/request_body.json", "2/requests/5/request_body.json"},
		{"/", ""},
		{"", ""},
	}

	for _, c := range cases {
		if got := normalizeObjectKey(c.in); got != c.want {
			t.Errorf("normalizeObjectKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
