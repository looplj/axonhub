package xerrors

import "errors"

func As[T error](rawErr error) (T, bool) {
	err := new(T)
	ok := errors.As(rawErr, err)

	return *err, ok
}

func As0[T error](rawErr error) bool {
	err := new(T)
	ok := errors.As(rawErr, err)

	return ok
}

func Is(err, target error) bool {
	return errors.Is(err, target)
}

func IsNot(err, target error) bool {
	return !errors.Is(err, target)
}

func NoErr(err error) {
	if err != nil {
		panic(err)
	}
}

func NoErr2[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}

	return val
}
