package util

import (
	"container/list"
)

type Queue struct {
	q *list.List
}

func NewQueue() *Queue {
	return &Queue{list.New()}
}

func (oq *Queue) Push(order interface{}) *list.Element {
	return oq.q.PushBack(order)
}

func (oq *Queue) Pop() interface{} {
	front := oq.q.Front()
	if front == nil {
		return nil
	}

	return oq.q.Remove(front)
}

func (oq *Queue) Remove(elem *list.Element) interface{} {
	if elem == nil {
		return nil
	}
	return oq.q.Remove(elem)
}

// Front returns the first element without removing it.
func (oq *Queue) Front() *list.Element {
	return oq.q.Front()
}

func (oq *Queue) Len() int {
	return oq.q.Len()
}

func (oq *Queue) ForEach(fn func(v interface{})) {
	for e := oq.q.Front(); e != nil; e = e.Next() {
		fn(e.Value)
	}
}
