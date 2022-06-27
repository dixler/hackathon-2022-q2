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

func storeResource(stackName backend.StackSummary, resState resource.State) error {
	fmt.Printf("[%s] %s %s\n", stackName.Name(), resState.Type.String(), resState.URN.Name())
	return nil
}

func handleStack(stackName backend.StackSummary, b httpstate.Backend, q Query, ctx context.Context) error {
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
		storeResource(stackName, *resState)
	}
	return nil
}

func isResourceType(query string) bool {
	return strings.Count(query, ":") > 0
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

func getAllRedshift() {
	/*
		url := fmt.Sprintf("sslmode=require user=%v password=%v host=%v port=%v dbname=%v",
			username,
			password,
			host,
			port,
			dbName)

		var err error
		var db *sql.DB
		if db, err = sql.Open("postgres", url); err != nil {
			return nil, fmt.Errorf("redshift connect error : (%v)"), err
		}

		if err = db.Ping(); err != nil {
			return nil, fmt.Errorf("redshift ping error : (%v)", err)
		}
		return db, nil
	*/
}

func getAll(q Query) {
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
	ch := make(chan bool, len(stacks))
	for _, stackName := range stacks {
		go func(stackName backend.StackSummary) {
			err = handleStack(stackName, b, q, ctx)
			if err != nil {
				//fmt.Println(err)
			}
			ch <- true
		}(stackName)
	}
	for range stacks {
		_, more := <-ch
		if !more {
			panic("what")
		}
	}
	close(ch)
}
func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "gets resources on the pulumi service",
		Args:  cmdutil.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			querystring := args[0]

			// handle resource types
			/*

				curl \
					-H "Accept: application/vnd.pulumi+8" \
					-H "Content-Type: application/json" \
					-H "Authorization: token $PULUMI_ACCESS_TOKEN" \
					https://api.pulumi.com/api/stacks/{organization}/{project}/{stack}/export
			*/

			// handle stack queries
			q := parseQuery(querystring)
			getAll(q)
		},
	}
	return cmd
}
