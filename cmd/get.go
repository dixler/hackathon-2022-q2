package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/go/common/diag"
	"github.com/pulumi/pulumi/sdk/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/go/common/resource"
	"github.com/spf13/cobra"
)

func isResourceMatch(res resource.State, q Query) bool {
	resTypeParts := strings.Split(res.Type.String(), ":")

	if len(resTypeParts) != 3 {
		return false
	}

	provider := resTypeParts[0]
	module := resTypeParts[1]
	name := resTypeParts[2]

	if q.ResourceType.Provider != "" && provider != q.ResourceType.Provider {
		return false
	}
	if q.ResourceType.ModulePrefix != "" && !strings.HasPrefix(module, q.ResourceType.ModulePrefix) {
		return false
	}
	if q.ResourceType.Module != "" && module != q.ResourceType.Module {
		return false
	}
	if q.ResourceType.Resource != "" && name != q.ResourceType.Resource {
		return false
	}
	return true
}

func storeResource(stackName backend.StackSummary, resState resource.State, q Query, props []Prop, out chan []string) error {
	if !isResourceMatch(resState, q) {
		return nil
	}
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

func numLen(num int) int {
	return len(fmt.Sprintf("%d", num))
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
	for _, resState := range snap.Resources {
		if resState == nil {
			continue
		}
		storeResource(stackName, *resState, q, ps, out)
	}
	return nil
}

func getAll(q Query, ps []Prop, flags getFlags) {
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
	filter := backend.ListStacksFilter{}
	if q.StackReference.Org != "" {
		filter.Organization = &q.StackReference.Org
	}
	if q.StackReference.Project != "" {
		filter.Project = &q.StackReference.Project
	}

	stacks, err := b.ListStacks(ctx, filter)
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
				processed <- true
			}(stackName)
		}
		for i := 0; i < len(stacks); i++ {
			<-processed
		}
		close(processed)
		close(ch)
	}()

	total := 0
	stackCounter := make(map[string]int)
	resourceCounter := make(map[string]int)

	for line := range ch {
		//line, more := <-ch
		for _, elem := range line {
			fmt.Print(elem, "\t\t")
		}
		fmt.Println()

		stackRefString := line[0]
		stackRef := strings.Split(stackRefString, "/")
		org := stackRef[0]
		project := stackRef[1]
		stack := stackRef[2]

		//
		stackCounter[org+"/"] += 1
		stackCounter[org+"/"+project+"/"] += 1
		stackCounter[org+"/"+project+"/"+stack] += 1

		resourceTypeString := line[1]
		resourceType := strings.Split(resourceTypeString, ":")
		provider := resourceType[0]
		module := resourceType[1]
		name := resourceType[2]

		// handle provider
		resourceCounter[provider+":"] += 1

		// handle module
		modulePaths := strings.Split(module, "/")
		for m := range modulePaths {
			moduleKey := strings.Join(modulePaths[0:m+1], "/")
			resourceCounter[provider+":"+moduleKey+":"] += 1
		}

		// handle name
		resourceCounter[provider+":"+module+":"+name] += 1

		total += 1
	}

	if !flags.Summarize {
		return
	}
	fmt.Println()
	fmt.Println("Summary")
	fmt.Println("total", "-", total)

	{
		fmt.Println()
		fmt.Println("Summary[by-stack]")
		fmt.Printf("group  count stack\n")
		max_width := 0
		max_width_count := 0
		keys := make([]string, 0, len(stackCounter))
		for k, count := range stackCounter {
			keys = append(keys, k)
			if len(k) > max_width {
				max_width = len(k)
			}
			if numLen(count) > max_width_count {
				max_width_count = numLen(count)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts := 0
			for _, p := range strings.Split(k, "/") {
				if p == "" {
					continue
				}
				parts += 1
			}
			indentLevel := "|"
			indent := strings.Repeat(indentLevel, parts)

			count := stackCounter[k]
			padding := strings.Repeat(" ", (max_width_count - numLen(count)))

			fmt.Printf("stack: %d%s %s %s\n", count, padding, indent, k)
		}
	}
	{
		fmt.Println()
		fmt.Println("Summary[by-resource-type]")
		fmt.Printf("group  count resource-type\n")
		max_width := 0
		max_width_count := 0
		keys := make([]string, 0, len(resourceCounter))
		for k, count := range resourceCounter {
			// hack sorting by changing : to a higher priority character than /
			k = strings.ReplaceAll(k, ":", "\"")
			keys = append(keys, k)
			if len(k) > max_width {
				max_width = len(k)
			}
			if numLen(count) > max_width_count {
				max_width_count = numLen(count)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			k = strings.ReplaceAll(k, "\"", ":")

			colons := strings.Count(k, ":")
			subpaths := strings.Count(k, "/")
			indentLevel := "|"
			indent := strings.Repeat(indentLevel, colons+subpaths)

			count := resourceCounter[k]
			padding := strings.Repeat(" ", (max_width_count - numLen(count)))
			fmt.Printf("type: %d%s %s %s\n", count, padding, indent, k)
		}
	}
}

type getFlags struct {
	Summarize bool
	//byStack        bool
	//byResourceType bool
}

func newGetCmd() *cobra.Command {
	flags := getFlags{}
	cmd := &cobra.Command{
		Use:   "get",
		Short: "gets resources on the pulumi service",
		Run: func(cmd *cobra.Command, args []string) {
			q, p, err := parseArgs(args)
			if err != nil {
				fmt.Println(err)
			}
			getAll(q, p, flags)
		},
	}
	cmd.PersistentFlags().BoolVarP(
		&flags.Summarize, "summarize", "", false,
		"Summarize resource counts")
	return cmd
}
