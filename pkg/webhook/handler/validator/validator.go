package validator

import "context"

type Validator[T any] interface {
	Validate(ctx context.Context, obj T) (err error)
}
