package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/aipclient"
)

func apiClient() *aipclient.Client {
	client, err := aipclient.New(os.Getenv("AIP_API_URL"), os.Getenv("AIP_ORG_ID"), os.Getenv("AIP_API_TOKEN"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "aipctl:", err)
		os.Exit(2)
	}
	return client
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func runTasks(args []string) {
	fs := flag.NewFlagSet("tasks", flag.ExitOnError)
	project := fs.String("project", "", "project id")
	_ = fs.Parse(args)
	if *project == "" {
		fmt.Fprintln(os.Stderr, "aipctl tasks: --project is required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tasks, err := apiClient().ListTasks(ctx, *project)
	if err != nil {
		fmt.Fprintln(os.Stderr, "aipctl tasks:", err)
		os.Exit(1)
	}
	printJSON(tasks)
}

func runStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	project := fs.String("project", "", "project id")
	task := fs.String("task", "", "task ref (T-12)")
	agent := fs.String("agent", "development", "agent type")
	_ = fs.Parse(args)
	if *project == "" || *task == "" {
		fmt.Fprintln(os.Stderr, "aipctl start: --project and --task are required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	run, err := apiClient().StartRun(ctx, *project, *task, *agent)
	if err != nil {
		fmt.Fprintln(os.Stderr, "aipctl start:", err)
		os.Exit(1)
	}
	printJSON(run)
}

func runShow(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: aipctl run <run-id>")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	run, err := apiClient().GetRun(ctx, fs.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "aipctl run:", err)
		os.Exit(1)
	}
	printJSON(run)
}

func runApprove(args []string) {
	fs := flag.NewFlagSet("approve", flag.ExitOnError)
	decision := fs.String("decision", "approved", "approved or rejected")
	reason := fs.String("reason", "", "decision note")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: aipctl approve <approval-id> [--decision approved] [--reason ...]")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := apiClient().Decide(ctx, fs.Arg(0), *decision, *reason); err != nil {
		fmt.Fprintln(os.Stderr, "aipctl approve:", err)
		os.Exit(1)
	}
	fmt.Println("decision recorded")
}
