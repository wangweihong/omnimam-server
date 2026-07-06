package errors

import (
	"github.com/wangweihong/gotoolbox/pkg/sets"
)

// Aggregate represents an object that contains multiple errors, but does not
// necessarily have singular semantic meaning.
type Aggregate interface {
	error
	Errors() []error
}

// NewAggregate converts a slice of errors into an Aggregate interface, which
// is itself an implementation of the error interface.  If the slice is empty,
// this returns nil.
// It will check if any of the element of input error list is nil, to avoid
// nil pointer panic when call Error().
func NewAggregate(errList ...error) Aggregate {
	if len(errList) == 0 {
		return nil
	}
	// In case of input error list contains nil
	var errs []error
	for _, e := range errList {
		if e != nil {
			errs = append(errs, e)
		}
	}
	if len(errs) == 0 {
		return nil
	}

	return aggregate(errs)
}

// This helper implements the error and Errors interfaces.  Keeping it private
// prevents people from making an aggregate of 0 errors, which is not
// an error, but does satisfy the error interface.
type aggregate []error

// Error is part of the error interface.
func (agg aggregate) Error() string {
	if len(agg) == 0 {
		// This should never happen, really.
		return ""
	}
	if len(agg) == 1 {
		return agg[0].Error()
	}
	seenErrors := sets.NewString()
	result := ""

	for _, withsStack := range agg {
		msg := withsStack.Error()
		if seenErrors.Has(msg) {
			continue
		}
		seenErrors.Insert(msg)
		if len(seenErrors) > 1 {
			result += ", "
		}
		result += msg
	}

	if len(seenErrors) == 1 {
		return result
	}
	return "[" + result + "]"
}

// Errors is part of the Aggregate interface.
func (agg aggregate) Errors() []error {
	var es []error //nolint: prealloc
	for _, e := range agg {
		es = append(es, e)
	}
	return es
}

// Matcher is used to match errors.  Returns true if the error matches.
type Matcher func(error) bool

// FilterOut removes all errors that match any of the matchers from the input
// error.  If the input is a singular error, only that error is tested.  If the
// input implements the Aggregate interface, the list of errors will be
// processed recursively.
//
// This can be used, for example, to remove known-OK errors (such as io.EOF or
// os.PathNotFound) from a list of errors.
func FilterOut(err error, fns ...Matcher) error {
	if err == nil {
		return nil
	}
	if agg, ok := err.(Aggregate); ok { //nolint: errorlint
		return NewAggregate(filterErrors(agg.Errors(), fns...)...)
	}
	if !matchesError(err, fns...) {
		return err
	}
	return nil
}

// matchesError returns true if any Matcher returns true.
func matchesError(err error, fns ...Matcher) bool {
	for _, fn := range fns {
		if fn(err) {
			return true
		}
	}
	return false
}

// filterErrors returns any errors (or nested errors, if the list contains
// nested Errors) for which all fns return false. If no errors
// remain a nil list is returned. The resulting slice will have all
// nested slices flattened as a side effect.
func filterErrors(list []error, fns ...Matcher) []error {
	var result []error
	for _, err := range list {
		r := FilterOut(err, fns...)
		if r != nil {
			result = append(result, err)
		}
	}
	return result
}

// Flatten takes an Aggregate, which may hold other Aggregates in arbitrary
// nesting, and flattens them all into a single Aggregate, recursively.
func Flatten(agg Aggregate) Aggregate {
	result := []error{}
	if agg == nil {
		return nil
	}
	for _, err := range agg.Errors() {
		if a, ok := err.(Aggregate); ok { //nolint: errorlint
			r := Flatten(a)
			if r != nil {
				result = append(result, r.Errors()...)
			}
		} else {
			if err != nil {
				result = append(result, err)
			}
		}
	}
	return NewAggregate(result...)
}
