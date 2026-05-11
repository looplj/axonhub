package pipeline

import "errors"

// UpstreamError marks errors that originate from the upstream provider path.
type UpstreamError struct {
	Err error
}

func (e *UpstreamError) Error() string {
	if e == nil || e.Err == nil {
		return "upstream error"
	}

	return e.Err.Error()
}

func (e *UpstreamError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

func WrapUpstreamError(err error) error {
	if err == nil {
		return nil
	}

	var upstreamErr *UpstreamError
	if errors.As(err, &upstreamErr) {
		return err
	}

	return &UpstreamError{Err: err}
}

func IsUpstreamError(err error) bool {
	var upstreamErr *UpstreamError
	return errors.As(err, &upstreamErr)
}
