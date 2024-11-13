package model

import "strings"

type SliceFlag []string

func (i *SliceFlag) String() string {
	return strings.Join(*i, ", ")
}

func (i *SliceFlag) Set(value string) error {
	*i = append(*i, value)
	return nil
}
