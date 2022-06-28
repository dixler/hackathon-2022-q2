package main

import (
	"testing"
)

func TestGetStackResourceOutputs(t *testing.T) {
	t.Parallel()
	//getAll(parseQuery("pulumi/"))
	//getAll(parseQuery("aws:"), parseProps("bucket"))
	getAll(parseQuery("aws:"), parseProps(""))
}
