package util

func ApplyConversion[T any, K any](models []T, converter func(T) K) []K {
	dtos := make([]K, len(models))
	for k, v := range models {
		dtos[k] = converter(v)
	}

	return dtos
}
