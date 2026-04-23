package gql

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

const jsMaxSafeInteger = int64(9007199254740991)

type Int64Scalar int64

func (Int64Scalar) ImplementsGraphQLType(name string) bool {
	return name == "Int64"
}

func (i *Int64Scalar) MarshalGQL(w io.Writer) {
	if i == nil {
		w.Write([]byte("null"))
		return
	}
	v := int64(*i)
	if v > jsMaxSafeInteger || v < -jsMaxSafeInteger {
		fmt.Fprintf(w, `"%d"`, v)
	} else {
		fmt.Fprintf(w, `%d`, v)
	}
}

func (i *Int64Scalar) UnmarshalGQL(v interface{}) error {
	switch v := v.(type) {
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return err
		}
		*i = Int64Scalar(n)
		return nil
	case int:
		*i = Int64Scalar(v)
		return nil
	case int64:
		*i = Int64Scalar(v)
		return nil
	case *int64:
		if v != nil {
			*i = Int64Scalar(*v)
		}
		return nil
	case *int:
		if v != nil {
			*i = Int64Scalar(*v)
		}
		return nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return err
		}
		*i = Int64Scalar(n)
		return nil
	default:
		return fmt.Errorf("Int64 scalar must be a string or number, got %T", v)
	}
}
