package main

import (
	"testing"
)

func TestGetStackResourceOutputs(t *testing.T) {
	t.Parallel()
	//getAll(parseQuery("pulumi/"))
	//getAll(parseQuery("aws:"), parseProps("bucket"))
	//getAll(parseQuery("aws:"), parseProps(""))
	getAll(parseQuery("dixler/"), parseProps(""))
}

type QueryStringTest struct {
	Query   string
	IsValid bool
}

var QueryTests = []QueryStringTest{
	{
		Query:   "aws",
		IsValid: false,
	},
	{
		Query:   "aws,org/project/stack",
		IsValid: false,
	},
	{
		Query:   "aws:",
		IsValid: true,
	},
	{
		Query:   "aws:foo/bar/baz",
		IsValid: true,
	},
	{
		Query:   "aws:foo/",
		IsValid: true,
	},
	{
		Query:   "org/project/stack",
		IsValid: true,
	},
	{
		Query:   "org/project",
		IsValid: true,
	},
	{
		Query:   "org/",
		IsValid: true,
	},
	{
		Query:   "org/this:that/thing",
		IsValid: true, // TODO get input on this
	},
}

func TestQueryStringValid(t *testing.T) {
	for _, test := range QueryTests {
		if !(isQueryString(test.Query) == test.IsValid) {
			t.Fatalf("%v %v mismatch", test.Query, test.IsValid)
			t.Fail()
		}
	}
}
