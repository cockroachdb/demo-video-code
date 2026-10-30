package random

import (
	"math/rand"
)

// WeightedItem represents an item to be used in weighted set of items.
type WeightedItem struct {
	Value  string
	Weight int
}

// WeightedItems represents a weighted set of items.
type WeightedItems struct {
	items       []WeightedItem
	totalWeight int
}

// MakeWeightedItems returns a new WeightedItem set.
func MakeWeightedItems(items ...WeightedItem) WeightedItems {
	wi := WeightedItems{
		items: items,
	}

	for _, item := range items {
		wi.totalWeight += item.Weight
	}

	return wi
}

// Choose returns a random item from the weighted set.
func (wi WeightedItems) Choose() string {
	randomWeight := rand.Intn(wi.totalWeight-1) + 1
	for _, i := range wi.items {
		randomWeight -= i.Weight
		if randomWeight <= 0 {
			return i.Value
		}
	}

	return ""
}
