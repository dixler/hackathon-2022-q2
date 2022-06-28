package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/go/common/diag"
	"github.com/pulumi/pulumi/sdk/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/go/common/resource"
	"github.com/pulumi/pulumi/sdk/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func storeResource(stackName backend.StackSummary, resState resource.State, props []Prop, out chan []string) error {
	resourceProps := resState.Outputs.Mappable()

	line := make([]string, 3+len(props))
	line[0] = stackName.Name().String()
	line[1] = resState.Type.String()
	line[2] = resState.URN.Name().String()

	for i, prop := range props {
		val, ok := resourceProps[prop.Name]
		if !ok {
			return nil
		}
		line[3+i] = fmt.Sprintf("%s ", val)
	}
	out <- line
	return nil
}

func handleStack(stackName backend.StackSummary, b httpstate.Backend, q Query, ps []Prop, out chan []string, ctx context.Context) error {
	stk, err := b.GetStack(ctx, stackName.Name())
	httpStack := stk.(httpstate.Stack)

	if q.StackReference.Org != "" && httpStack.StackIdentifier().Owner != q.StackReference.Org {
		return nil
	}
	if q.StackReference.Project != "" && httpStack.StackIdentifier().Project != q.StackReference.Project {
		return nil
	}
	if q.StackReference.Stack != "" && httpStack.StackIdentifier().Stack != q.StackReference.Stack {
		return nil
	}

	if err != nil {
		return fmt.Errorf("error retrieving stack: %s", stackName.Name())
	}
	snap, err := stk.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("error retrieving stack: %s", stackName.Name())
	}
	//conf, err := b.Client().GetStackUpdates(ctx, httpStack.StackIdentifier())
	//if err != nil {
	//	return fmt.Errorf("error retrieving stack: %s", stackName.Name())
	//}
	//author := "<none>"
	//if len(conf) > 0 {
	//	fmt.Println(conf[len(conf)-1].Config)
	//}
	for _, resState := range snap.Resources {
		if resState == nil {
			continue
		}
		resTypeParts := strings.Split(resState.Type.String(), ":")

		if len(resTypeParts) != 3 {
			continue
		}

		provider := resTypeParts[0]
		module := resTypeParts[1]
		name := resTypeParts[2]

		if q.ResourceType.Provider != "" && provider != q.ResourceType.Provider {
			continue
		}
		if q.ResourceType.Module != "" && module != q.ResourceType.Module {
			continue
		}
		if q.ResourceType.Resource != "" && name != q.ResourceType.Resource {
			continue
		}
		storeResource(stackName, *resState, ps, out)
	}
	return nil
}

func isResourceType(query string) bool {
	return strings.Count(query, ":") > 0
}

func parseProps(propstring string) []Prop {
	if propstring == "" {
		return []Prop{}
	}

	ps := strings.Split(propstring, ",")

	props := make([]Prop, len(ps))

	for i, prop := range ps {
		props[i] = Prop{
			Name: prop,
		}
	}
	return props
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
			rtQuery.Module = resourceParts[1]
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

type StackRefQuery struct {
	Org     string
	Project string
	Stack   string
}

type ResourceTypeQuery struct {
	Provider string
	Module   string // pathlike
	Resource string
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

func getAll(q Query, ps []Prop) {
	// <org>/<project>/<stack>
	sink := diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
		Color: colors.Raw,
	})
	b, err := httpstate.New(sink, httpstate.DefaultURL())
	if err != nil {
		fmt.Printf("%s", err.Error())
		return
	}
	ctx := context.Background()
	// TODO this initial data load is truncated
	stacks, err := b.ListStacks(ctx, backend.ListStacksFilter{})
	if err != nil {
		fmt.Printf("%s", err.Error())
		return
	}
	ch := make(chan []string, 100)
	go func() {
		processed := make(chan bool, len(stacks))
		for _, stackName := range stacks {
			go func(stackName backend.StackSummary) {
				err = handleStack(stackName, b, q, ps, ch, ctx)
				if err != nil {
					//fmt.Println(err)
				}
				processed <- true
			}(stackName)
		}
		for range stacks {
			_, more := <-processed
			if !more {
				panic("what")
			}
		}
		close(ch)
	}()

	lines := [][]string{}
	colwidth := make([]int, 3+len(ps))
	for {
		line, more := <-ch
		for i, elem := range line {
			curLength := len(elem)
			if colwidth[i] >= curLength {
				continue
			}
			colwidth[i] = curLength
		}
		lines = append(lines, line)
		if !more {
			break
		}
	}
	for _, line := range lines {
		for i, elem := range line {
			width := colwidth[i]
			fmt.Print(elem, strings.Repeat(" ", width-len(elem)), " ")
		}
		fmt.Println()
	}
}

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "gets resources on the pulumi service",
		Args:  cmdutil.MaximumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			querystring := ""
			propstring := ""
			switch len(args) {
			case 2:
				propstring = args[1]
				fallthrough
			case 1:
				querystring = args[0]
			}
			// handle stack queries
			getAll(parseQuery(querystring), parseProps(propstring))
		},
	}
	return cmd
}
