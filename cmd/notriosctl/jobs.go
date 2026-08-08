package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/renesugar/notrios/internal/jobs"
	"github.com/renesugar/notrios/internal/store"
)

// Exit codes for `notriosctl jobs status`.
//
// Designed rather than accreted, because a script that cannot tell "not
// finished" from "failed" will either poll forever or give up early. These are
// what `job-a && job-b` needs, and they are why no scheduler is required.
const (
	exitJobSucceeded = 0
	exitJobFailed    = 1
	exitJobUsage     = 2 // matches lint, fix, and every other command here
	exitJobRunning   = 3
	exitJobCancelled = 4
	exitJobUnknown   = 5
	// exitJobInterrupted is separate from failed on purpose: the work may have
	// completed a great deal before the process died, and the importers resume
	// from their own checkpoints, so "run it again" is the right response —
	// which is not the right response to a failure.
	exitJobInterrupted = 6
)

// jobExitCode maps a settled or live state to the contract above.
func jobExitCode(state string) int {
	switch state {
	case store.JobSucceeded:
		return exitJobSucceeded
	case store.JobFailed:
		return exitJobFailed
	case store.JobCancelled:
		return exitJobCancelled
	case store.JobInterrupted:
		return exitJobInterrupted
	default:
		return exitJobRunning
	}
}

// startTrackedJob records a run and arranges for Ctrl-C to cancel it.
//
// Without the signal handler, Ctrl-C would leave a record saying `running`
// until its heartbeat went stale — a lie for two minutes, and then only
// "interrupted". Catching the first signal turns it into the truth immediately;
// a second signal still kills the process, which is what an impatient operator
// means by pressing it twice.
func startTrackedJob(st store.Store, kind string, parameters []store.JobParameter) (*jobs.Runner, context.Context) {
	runner, ctx, err := jobs.Start(context.Background(), st, store.CreateJobRequest{
		Kind: kind, Parameters: parameters,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "job %s started (%s); watch it with: notriosctl jobs status %s --wait\n",
		runner.ID(), kind, runner.ID())

	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		fmt.Fprintf(os.Stderr, "\ncancelling job %s at the next checkpoint; press again to stop immediately\n", runner.ID())
		_ = runner.Cancel(context.Background())
		<-signals
		signal.Stop(signals)
		fmt.Fprintln(os.Stderr, "stopping now; the job record will show as interrupted")
		os.Exit(130)
	}()
	return runner, ctx
}

// finishTrackedJob settles the record and reports the outcome to the operator.
func finishTrackedJob(runner *jobs.Runner, summary map[string]any, failure error) {
	job, err := runner.Finish(summary, failure)
	if err != nil {
		fmt.Fprintf(os.Stderr, "job record could not be settled: %v\n", err)
	}
	switch {
	case failure == nil:
		fmt.Fprintf(os.Stderr, "job %s %s\n", runner.ID(), job.State)
	case job.State == store.JobCancelled:
		// Not a failure: the work stopped where it was asked to, at a
		// checkpoint, and running the same command again resumes.
		fmt.Fprintf(os.Stderr, "job %s cancelled; rerun the same command to continue\n", runner.ID())
	default:
		fmt.Fprintln(os.Stderr, failure)
		os.Exit(1)
	}
}

func runJobs(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl jobs list|status|show|cancel ...")
		os.Exit(exitJobUsage)
	}
	switch args[0] {
	case "list":
		runJobsList(args[1:])
	case "status":
		runJobsStatus(args[1:])
	case "show":
		runJobsShow(args[1:])
	case "cancel":
		runJobsCancel(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown jobs subcommand %q (want list, status, show or cancel)\n", args[0])
		os.Exit(exitJobUsage)
	}
}

func runJobsList(args []string) {
	fs := flag.NewFlagSet("notriosctl jobs list", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	kind := fs.String("kind", "", "only this job kind")
	state := fs.String("state", "", "only this state")
	limit := fs.Int("limit", 0, "maximum rows (0 = 50)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitJobUsage)
	}
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	list, err := st.ListJobs(context.Background(), store.JobListRequest{
		Kind: *kind, State: *state, Limit: *limit,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Parameters are withheld here as they are over REST: a listing is a
	// control-plane view, and a job's parameters name places on this machine.
	// `jobs show` is where they belong, because it is asked for one job.
	rows := make([]map[string]any, 0, len(list.Jobs))
	for _, job := range list.Jobs {
		rows = append(rows, jobSummaryRow(job))
	}
	printJSON(map[string]any{"jobs": rows, "truncated": list.Truncated})
}

func jobSummaryRow(job store.Job) map[string]any {
	row := map[string]any{
		"id": job.ID, "kind": job.Kind, "state": job.State,
		"processed": job.Processed, "total": job.Total,
		"created_at": job.CreatedAt.Format(time.RFC3339),
	}
	if job.Phase != "" {
		row["phase"] = job.Phase
	}
	if !job.FinishedAt.IsZero() {
		row["finished_at"] = job.FinishedAt.Format(time.RFC3339)
	}
	return row
}

// runJobsStatus is the shell-legible half of the control plane.
func runJobsStatus(args []string) {
	fs := flag.NewFlagSet("notriosctl jobs status", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	wait := fs.Bool("wait", false, "block until the job settles, then exit with its code")
	quiet := fs.Bool("quiet", false, "print nothing; report through the exit code only")
	timeout := fs.Duration("timeout", 0, "with --wait, give up after this long (0 = no limit)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitJobUsage)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl jobs status [--wait] [--timeout 30m] <job-id>")
		explainTrailingFlags(fs.Args())
		fs.PrintDefaults()
		os.Exit(exitJobUsage)
	}
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()

	ctx := context.Background()
	deadline := time.Time{}
	if *wait && *timeout > 0 {
		deadline = time.Now().Add(*timeout)
	}
	for {
		job, err := st.GetJob(ctx, fs.Arg(0))
		if err != nil {
			if !*quiet {
				fmt.Fprintln(os.Stderr, err)
			}
			os.Exit(exitJobUnknown)
		}
		// An interrupted job counts as settled: nothing will move it again, so
		// waiting for it would be waiting forever.
		if !*wait || job.Settled() {
			if !*quiet {
				printJSON(jobSummaryRow(job))
			}
			os.Exit(jobExitCode(job.State))
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			if !*quiet {
				fmt.Fprintf(os.Stderr, "job %s is still %s after %s\n", job.ID, job.State, *timeout)
			}
			os.Exit(exitJobRunning)
		}
		time.Sleep(time.Second)
	}
}

// explainTrailingFlags names the mistake a script author is most likely to
// make here.
//
// Go's flag package stops parsing at the first non-flag argument, so
// `jobs status <id> --wait` leaves `--wait` as a second positional and the
// command exits 2. That is standard and matches every other command here, but
// this is the one command written to be driven from a shell, so the refusal
// should say what to change rather than print a wall of defaults and stop.
func explainTrailingFlags(args []string) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			fmt.Fprintf(os.Stderr, "note: %s must come before the job ID; flags after it are read as arguments\n", arg)
			return
		}
	}
}

func runJobsShow(args []string) {
	fs := flag.NewFlagSet("notriosctl jobs show", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	asCommand := fs.Bool("command", false, "print the command that reproduces this run and nothing else")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitJobUsage)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl jobs show [--command] <job-id>")
		explainTrailingFlags(fs.Args())
		fs.PrintDefaults()
		os.Exit(exitJobUsage)
	}
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	job, err := st.GetJob(context.Background(), fs.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitJobUnknown)
	}
	if *asCommand {
		fmt.Println(renderJobCommand(job))
		return
	}
	row := jobSummaryRow(job)
	row["parameters"] = job.Parameters
	row["command"] = renderJobCommand(job)
	if job.Error != "" {
		row["error"] = job.Error
	}
	if len(job.Summary) > 0 {
		row["summary"] = job.Summary
	}
	if job.CancelRequested {
		row["cancel_requested"] = true
	}
	printJSON(row)
}

func runJobsCancel(args []string) {
	fs := flag.NewFlagSet("notriosctl jobs cancel", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitJobUsage)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl jobs cancel <job-id>")
		os.Exit(exitJobUsage)
	}
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	job, err := st.RequestJobCancel(context.Background(), fs.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitJobUnknown)
	}
	// Asking is not stopping: the work stops at its next durable boundary, so
	// the record still says `running` here and that is not a bug.
	printJSON(jobSummaryRow(job))
}

// jobCommands maps a kind to the command line that runs it.
var jobCommands = map[string][]string{
	store.JobKindImportJoplinRaw: {"import", "joplin-raw"},
	store.JobKindImportObsidian:  {"import", "obsidian"},
	store.JobKindExportArchiveV2: {"export", "archive-v2"},
}

// renderJobCommand turns stored parameters back into a runnable command line.
//
// Rendered rather than stored, which was a deliberate decision: storing raw
// argv would have captured whatever happened to be on the command line —
// including any secret — into the database, and a stored string cannot improve
// when a flag is renamed while a rendering can. A parameter whose name begins
// with `_` is positional and is placed last, in the order it was stored.
func renderJobCommand(job store.Job) string {
	parts := append([]string{"notriosctl"}, jobCommands[job.Kind]...)
	if len(jobCommands[job.Kind]) == 0 {
		// A kind this build does not know how to run. Saying so beats printing
		// a command that would not work.
		return "# no command is known for job kind " + job.Kind
	}
	positional := []string{}
	for _, parameter := range job.Parameters {
		if strings.HasPrefix(parameter.Name, "_") {
			positional = append(positional, shellQuote(parameter.Value))
			continue
		}
		if parameter.Value == "true" {
			parts = append(parts, "--"+parameter.Name)
			continue
		}
		if parameter.Value == "" || parameter.Value == "false" {
			continue
		}
		parts = append(parts, "--"+parameter.Name, shellQuote(parameter.Value))
	}
	return strings.Join(append(parts, positional...), " ")
}

// shellQuote makes a value safe to paste back into a shell.
func shellQuote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\n\"'\\$`*?[]{}();&|<>#~!") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
