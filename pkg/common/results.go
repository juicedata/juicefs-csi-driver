package common

import (
	"context"
	"k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type resultKind int

const (
	noRequeue resultKind = iota
	delayRequeue
	normalRequeue
)

type Results struct {
	ctx     context.Context
	current reconcile.Result
	kind    resultKind
	errors  []error
}

func NewResult(ctx context.Context) *Results {
	return &Results{
		ctx: ctx,
	}
}

func (r *Results) With(name string, fn func() (reconcile.Result, error)) *Results {
	log := ctrl.Log.WithName(name)
	log.Info("run step")

	result, err := fn()
	if err != nil {
		log.Error(err, "run step failed")
	}
	return r.WithError(err).WithResult(&Results{current: result, kind: kindOfResult(result)})
}

func (r *Results) WithResult(results *Results) *Results {
	switch {
	case results.kind > r.kind:
		r.kind = results.kind
		r.current = results.current
	case results.kind == r.kind && r.kind == delayRequeue:
		if results.current.RequeueAfter < r.current.RequeueAfter {
			r.current.RequeueAfter = results.current.RequeueAfter
		}
	}
	r.errors = append(r.errors, results.errors...)
	return r
}

func (r *Results) WithError(err error) *Results {
	if err != nil {
		r.errors = append(r.errors, err)
	}
	return r
}

func (r *Results) Aggregate() (reconcile.Result, error) {
	return r.current, errors.NewAggregate(r.errors)
}

func NewResults(ctx context.Context) *Results {
	return &Results{
		ctx:     ctx,
		current: reconcile.Result{},
		kind:    noRequeue,
		errors:  []error{},
	}
}

func kindOfResult(result reconcile.Result) resultKind {
	switch {
	case result.RequeueAfter > 0:
		return delayRequeue
	case result.Requeue && result.RequeueAfter == 0:
		return normalRequeue
	default:
		return noRequeue
	}
}
