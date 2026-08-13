package reports

import "errors"

// Request errors for the reporting slice. They live in the app package
// rather than a domain one because they are about the shape of a request,
// not about what the numbers mean.
var (
	// ErrInvalidRange means from/to are missing, equal, or inverted.
	ErrInvalidRange = errors.New("from must be before to")
	// ErrRangeTooLong means the window exceeds the reporting limit.
	ErrRangeTooLong = errors.New("reporting range may not exceed two years")
	// ErrInvalidGranularity means a bucket size outside the known set.
	ErrInvalidGranularity = errors.New("granularity must be day, week, or month")
)
