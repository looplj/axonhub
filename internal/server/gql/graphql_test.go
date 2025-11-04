package gql

import (
	"testing"

	"github.com/samber/lo"
)

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
			first:     lo.ToPtr(0),
			expectErr: "first must be greater than 0",
		},
		{
			name:      "first too large",
			first:     lo.ToPtr(1001),
			expectErr: "first cannot exceed 1000",
		},
		{
			name:      "last zero",
			last:      lo.ToPtr(0),
			expectErr: "last must be greater than 0",
		},
		{
			name:      "last too large",
			last:      lo.ToPtr(1001),
			expectErr: "last cannot exceed 1000",
		},
		{
			name:  "valid first",
			first: lo.ToPtr(10),
		},
		{
			name: "valid last",
			last: lo.ToPtr(5),
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
				_, err := r.APIKeys(t.Context(), nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Channels",
			fn: func() error {
				_, err := r.Channels(t.Context(), nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "DataStorages",
			fn: func() error {
				_, err := r.DataStorages(t.Context(), nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Projects",
			fn: func() error {
				_, err := r.Projects(t.Context(), nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Requests",
			fn: func() error {
				_, err := r.Requests(t.Context(), nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Roles",
			fn: func() error {
				_, err := r.Roles(t.Context(), nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Systems",
			fn: func() error {
				_, err := r.Systems(t.Context(), nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Threads",
			fn: func() error {
				_, err := r.Threads(t.Context(), nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Traces",
			fn: func() error {
				_, err := r.Traces(t.Context(), nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "UsageLogs",
			fn: func() error {
				_, err := r.UsageLogs(t.Context(), nil, nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Users",
			fn: func() error {
				_, err := r.Users(t.Context(), nil, nil, nil, nil, nil, nil)
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
