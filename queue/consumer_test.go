package queue

// consumerFunc adapts a plain function into a Consumer, so a test can declare one inline.
// It stays unexported on purpose -- see AGENTS.md.
type consumerFunc func(Task) Result

// OnPublish implements the Consumer interface
func (fn consumerFunc) OnPublish(*Task) error { return nil }

// Run implements the Consumer interface
func (fn consumerFunc) Run(task Task) Result { return fn(task) }

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
