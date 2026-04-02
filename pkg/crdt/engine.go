package crdt

type Operation interface {
}

type CRDTNode[State any] interface {
	Merge(other CRDTNode[State]) error

	ApplyOperation(op Operation) error

	State() State
}
