package main

import (
	"fmt"
	"strings"
)

func isResourceType(query string) bool {
	return strings.Count(query, ":") > 0
}

func parseProp(propstring string) Prop {
	return Prop{
		Name: propstring,
	}
}

var QUERY_STRING_BLACKLIST = []string{
	",",
	"=",
}

func isQueryString(s string) bool {
	for _, c := range QUERY_STRING_BLACKLIST {
		if strings.Contains(s, c) {
			return false
		}
	}

	numColons := strings.Count(s, ":")

	if 0 < numColons && numColons <= 2 {
		// Matches resource type pattern
		// Acceptable:
		//     provider:path/to/module:name
		//     provider:path/to/module
		//     provider:partial/path/ending/in/slash/
		//     provider:

		// TODO surprisingly matches
		//     a/b:c/d:d/f
		return true
	}

	if numColons > 0 {
		return false
	}

	numSlashes := strings.Count(s, "/")
	// No colons at this point
	if numSlashes > 0 && numSlashes <= 2 {
		// Matches stack reference pattern
		// Acceptable:
		//     org/project/stack
		//     org/project
		//     org/
		return true
	}

	return false
}

func parseQuery(p string) Query {
	/*
			urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucket:Bucket::my-bucket
		        ^^^^^^ ^^^^^^^^^^  ^^^^^^^^^^^^^^^^  ^^^^^^^^^^^^^^^^^^^^^^^^^ ^^^^^^^^^^^^^^^^^^^^  ^^^^^^^^^
		        <org>  <stack-name> <project-name>   <parent-type>             <resource-type>       <resource-name>
	*/
	query := Query{}
	rtQuery := &query.ResourceType
	srQuery := &query.StackReference
	if isResourceType(p) { // not a stack reference
		resourceParts := strings.Split(p, ":")

		switch len(resourceParts) {
		case 3:
			rtQuery.Resource = resourceParts[2]
			fallthrough
		case 2:
			module := resourceParts[1]
			if strings.HasSuffix(module, "/") {
				// is a prefix match
				rtQuery.ModulePrefix = resourceParts[1]
			} else {
				rtQuery.Module = resourceParts[1]
			}
			fallthrough
		case 1:
			rtQuery.Provider = resourceParts[0]
		}
	} else {
		// Assume is a stack reference
		stackParts := strings.Split(p, "/")

		switch len(stackParts) {
		case 3:
			srQuery.Stack = stackParts[2]
			fallthrough
		case 2:
			srQuery.Project = stackParts[1]
			fallthrough
		case 1:
			srQuery.Org = stackParts[0]
		}
	}
	return query
}

func parseArgs(args []string) (Query, []Prop, error) {
	q := Query{}
	p := []Prop{}

	queryStrings := []string{}
	for _, arg := range args {
		if !isQueryString(arg) {
			// TODO props
			p = append(p, parseProp(arg))
			continue
		}
		curQ := parseQuery(arg)
		if curQ.ResourceType.Provider != "" ||
			curQ.ResourceType.Module != "" ||
			curQ.ResourceType.ModulePrefix != "" ||
			curQ.ResourceType.Resource != "" {

			q.ResourceType = curQ.ResourceType
			queryStrings = append(queryStrings, arg)
		}
		if curQ.StackReference.Org != "" ||
			curQ.StackReference.Project != "" ||
			curQ.StackReference.Stack != "" {

			q.StackReference = curQ.StackReference
			queryStrings = append(queryStrings, arg)
		}
	}
	if len(queryStrings) > 2 {
		return Query{}, []Prop{}, fmt.Errorf("too many query strings provided: %v\n", queryStrings)
	}
	return q, p, nil
}

type StackRefQuery struct {
	Org     string
	Project string
	Stack   string
}

type ResourceTypeQuery struct {
	Provider     string
	Module       string // pathlike
	ModulePrefix string // pathlike must end with '/'
	Resource     string
}

type Query struct {
	StackReference StackRefQuery
	ResourceType   ResourceTypeQuery
}

type Cond struct {
	Operator string
	Args     []string
}
type Prop struct {
	Name string
	Cond Cond
}
