package streams

func All[T any](stream Stream[T]) ([]T, error) {
	var result []T

	for stream.Next() {
		result = append(result, stream.Current())
	}

	return result, stream.Err()
}
