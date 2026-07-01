package util

// MinPriceHeap orders price levels by lowest price first (min-heap).
type MinPriceHeap []*PriceLevel

func (h MinPriceHeap) Len() int { return len(h) }

func (h MinPriceHeap) Less(i, j int) bool { return h[i].Price < h[j].Price }

func (h MinPriceHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].Index, h[j].Index = i, j
}

func (h *MinPriceHeap) Push(x interface{}) {
	level := x.(*PriceLevel)
	level.Index = len(*h)
	*h = append(*h, level)
}

func (h *MinPriceHeap) Pop() interface{} {
	old := *h
	n := len(old)
	level := old[n-1]
	level.Index = -1
	*h = old[:n-1]
	return level
}

// Peek returns the top price level without removing it.
func (h MinPriceHeap) Peek() *PriceLevel {
	if len(h) == 0 {
		return nil
	}
	return h[0]
}
