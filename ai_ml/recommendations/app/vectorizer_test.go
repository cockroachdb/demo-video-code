package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyWeightings(t *testing.T) {
	v := []float32{0, 0, 0, 0, 0.77131015, 0.61704814, 0.15426204, 0.023139304}
	w := []float32{0.8, 0.8, 0.8, 0.8, 0.8, 0.5, 0.1, 0.1}
	v = applyWeightings(v, w)

	exp := []float32{0, 0, 0, 0, 0.61704814, 0.30852407, 0.015426204, 0.0023139303}
	assert.Equal(t, exp, v)
}
