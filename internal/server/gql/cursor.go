package gql

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/looplj/axonhub/internal/ent"
)

//nolint:errcheck,gosec // ignore error check.
func MarshalCursor(c ent.Cursor) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		quote := []byte{'"'}

		w.Write(quote)
		defer w.Write(quote)

		wc := base64.NewEncoder(base64.RawStdEncoding, w)
		defer wc.Close()

		_ = msgpack.NewEncoder(wc).Encode(c)
	})
}

// UnmarshalCursor convert a string to ent.Cursor.
// Unlike the Cursor.UnmarshalGQL, this function will transform the cursor value to UTC time.
// It is used to ensure the cursor value is in UTC time when query database.
func UnmarshalCursor(v any) (ent.Cursor, error) {
	var c ent.Cursor

	s, ok := v.(string)
	if !ok {
		return c, fmt.Errorf("%T is not a string", v)
	}

	if err := msgpack.NewDecoder(
		base64.NewDecoder(
			base64.RawStdEncoding,
			strings.NewReader(s),
		),
	).Decode(&c); err != nil {
		return c, fmt.Errorf("cannot decode cursor: %w", err)
	}

	if ts, ok := c.Value.(time.Time); ok {
		c.Value = ts.UTC()
	}

	return c, nil
}
