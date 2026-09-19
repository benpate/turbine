package queue

// consumerFunc adapts a plain function into a Consumer, so that a test can declare one inline.
//
// RULE: This is test-only on purpose.  Exported as an embeddable base, it would let a real
// Consumer whose hook name or signature drifted fall through to a silent no-op -- exactly what
// the required-method interface exists to prevent.
type consumerFunc func(Task) Result

// OnPublish implements the Consumer interface
func (fn consumerFunc) OnPublish(*Task) error { return nil }

// Consume implements the Consumer interface
func (fn consumerFunc) Consume(task Task) Result { return fn(task) }

// OnSuccess implements the Consumer interface
func (fn consumerFunc) OnSuccess(Task) error { return nil }

// OnError implements the Consumer interface
func (fn consumerFunc) OnError(Task, error) error { return nil }

// OnFailure implements the Consumer interface
func (fn consumerFunc) OnFailure(Task, error) error { return nil }

// withConsumerFuncs registers plain functions as Consumers, so that a test reads the same way
// it did before Consumer became an interface.
func withConsumerFuncs(consumers ...consumerFunc) Option {

	result := make([]Consumer, 0, len(consumers))

	for _, consumer := range consumers {
		result = append(result, consumer)
	}

	return WithConsumers(result...)
}
