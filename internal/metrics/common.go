package metrics

import "errors"

// ErrMetricMissing is returned when metric reader (lower level component)
// is missing and metric won't be available during process lifetime.
var ErrMetricMissing error = errors.New("metric is missing")
