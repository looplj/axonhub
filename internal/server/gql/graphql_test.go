package gql

import "testing"

func TestValidatePaginationArgs(t *testing.T) {
	cases := []struct {
		name      string
		first     *int
		last      *int
		expectErr string
	}{
		{
			name:      "missing both",
			expectErr: "either first or last must be provided",
		},
		{
			name:      "first zero",
			first:     ptr(0),
			expectErr: "first must be greater than 0",
		},
		{
			name:      "first too large",
			first:     ptr(1001),
			expectErr: "first cannot exceed 1000",
		},
		{
			name:      "last zero",
			last:      ptr(0),
			expectErr: "last must be greater than 0",
		},
		{
			name:      "last too large",
			last:      ptr(1001),
			expectErr: "last cannot exceed 1000",
		},
		{
			name:  "valid first",
			first: ptr(10),
		},
		{
			name: "valid last",
			last: ptr(5),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePaginationArgs(tc.first, tc.last)
			if tc.expectErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				return
			}

			if err == nil {
				t.Fatalf("expected error %q, got nil", tc.expectErr)
			}

			if err.Error() != tc.expectErr {
				t.Fatalf("expected error %q, got %q", tc.expectErr, err.Error())
			}
		})
	}
}

func TestQueryResolversRequirePagination(t *testing.T) {
	r := &queryResolver{&Resolver{}}

	testCases := []struct {
		name string
		fn   func() error
	}{
		{
			name: "APIKeys",
			fn: func() error {
				_, err := r.APIKeys(nil, nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Channels",
			fn: func() error {
				_, err := r.Channels(nil, nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "DataStorages",
			fn: func() error {
				_, err := r.DataStorages(nil, nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Projects",
			fn: func() error {
				_, err := r.Projects(nil, nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Requests",
			fn: func() error {
				_, err := r.Requests(nil, nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Roles",
			fn: func() error {
				_, err := r.Roles(nil, nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Systems",
			fn: func() error {
				_, err := r.Systems(nil, nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Threads",
			fn: func() error {
				_, err := r.Threads(nil, nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Traces",
			fn: func() error {
				_, err := r.Traces(nil, nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "UsageLogs",
			fn: func() error {
				_, err := r.UsageLogs(nil, nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Users",
			fn: func() error {
				_, err := r.Users(nil, nil, nil, nil, nil, nil, nil)
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn()
			if err == nil {
				t.Fatalf("expected pagination validation error, got nil")
			}

			if got := err.Error(); got != "either first or last must be provided" {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func ptr(v int) *int {
	return &v
}
