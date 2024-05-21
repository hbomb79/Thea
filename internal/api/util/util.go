package util

// ApplyOptionalConversion takes a pointer to a slice of models and will
// return nil if the pointer is nil, else it will dereference the pointer
// and using [ApplyConversion] to apply the converter function
// over all the elements in the slice.
func ApplyOptionalConversion[T any, K any](models *[]T, converter func(T) K) *[]K {
	if models == nil {
		return nil
	}

	out := ApplyConversion(*models, converter)
	return &out
}

// ApplyConversion applies a converter function to each of the models
// provided to this function. The returned value is a slice which
// has been converted to the new values based on the returned value
// from the converter.
//
// This function will explode if the models slice provided is nil. For nil-safety,
// consider using [ApplyOptionalConversion].
func ApplyConversion[T any, K any](models []T, converter func(T) K) []K {
	dtos := make([]K, 0, len(models))
	for _, v := range models {
		dtos = append(dtos, converter(v))
	}

	return dtos
}

// NotNilOrDefault expects a pointer to some type. If the pointer is
// nil, then the dflt value is returned. If the pointer is NOT nil, then
// it is dereferenced and the concrete value is returned.
func NotNilOrDefault[T any](maybe *T, dflt T) T {
	if maybe == nil {
		return dflt
	}

	return *maybe
}
