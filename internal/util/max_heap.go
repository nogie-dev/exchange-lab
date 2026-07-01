package util

type MaxPriceHeap []*PriceLevel

func (h MaxPriceHeap) Len() int { return len(h) }

func (h MaxPriceHeap) Less(i, j int) bool { return h[i].Price > h[j].Price }

func (h MaxPriceHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].Index, h[j].Index = i, j
}

func (h *MaxPriceHeap) Push(x interface{}) {
	level := x.(*PriceLevel)
	level.Index = len(*h)
	*h = append(*h, level)
}

func (h *MaxPriceHeap) Pop() interface{} {
	old := *h
	n := len(old)
	level := old[n-1]
	level.Index = -1
	*h = old[:n-1]
	return level
}

func (h MaxPriceHeap) Peek() *PriceLevel {
	if len(h) == 0 {
		return nil
	}
	return h[0]
}
