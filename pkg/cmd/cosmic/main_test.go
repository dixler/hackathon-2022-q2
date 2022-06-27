package main

import (
	"testing"
)

func TestGetStackResourceOutputs(t *testing.T) {
	t.Parallel()
	//getAll(parseQuery("dixler/"))
	getAll(parseQuery("aws:"))
}
