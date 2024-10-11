package queue

type Consumer func(name string, args map[string]any) (bool, error)
