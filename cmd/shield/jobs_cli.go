package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pborman/uuid"
	"github.com/spf13/cobra"

	. "github.com/starkandwayne/shield/api"
	"github.com/starkandwayne/shield/tui"
)

var (

	//== Applicable actions for Jobs
	createJobCmd = &cobra.Command{
		Use:   "job",
		Short: "Creates a new job",
		Long:  "Create a new job with ...",
	} // FIXME

	listJobCmd = &cobra.Command{
		Use:   "jobs",
		Short: "Lists all the jobs",
	}

	deleteJobCmd = &cobra.Command{
		Use:   "job",
		Short: "Deletes the specified job",
	}

	pauseJobCmd = &cobra.Command{
		Use:   "job",
		Short: "Pauses the specified job",
	}

	runJobCmd = &cobra.Command{
		Use:   "job",
		Short: "Runs the specified job",
	}

	editJobCmd = &cobra.Command{
		Use:   "job",
		Short: "Edit the specified job",
	}
)

func init() {

	// Hookup functions to the subcommands
	createJobCmd.Run = processCreateJobRequest
	deleteJobCmd.Run = processDeleteJobRequest
	runJobCmd.Run = processRunJobRequest
	editJobCmd.Run = processEditJobRequest

	// Add the subcommands to the base actions
	createCmd.AddCommand(createJobCmd)
	deleteCmd.AddCommand(deleteJobCmd)
	runCmd.AddCommand(runJobCmd)
	editCmd.AddCommand(editJobCmd)
}

type ListJobOptions struct {
	Unpaused  bool
	Paused    bool
	Target    string
	Store     string
	Schedule  string
	Retention string
	UUID      string
}

func ListJobs(opts ListJobOptions) error {
	jobs, err := GetJobs(JobFilter{
		Paused:    MaybeBools(opts.Unpaused, opts.Paused),
		Target:    opts.Target,
		Store:     opts.Store,
		Schedule:  opts.Schedule,
		Retention: opts.Retention,
	})
	if err != nil {
		return fmt.Errorf("\nERROR: Unexpected arguments following command: %v\n", err)
	}
	t := tui.NewTable("UUID", "P?", "Name", "Description", "Retention Policy", "Schedule", "Target", "Agent")
	for _, job := range jobs {
		paused := "-"
		if job.Paused {
			paused = "Y"
		}

		if len(opts.UUID) > 0 && opts.UUID == job.UUID {
			t.Row(job.UUID, paused, job.Name, job.Summary,
				job.RetentionName, job.ScheduleName, job.TargetEndpoint, job.Agent)
			break
		} else if len(opts.UUID) > 0 && opts.UUID != job.UUID {
			continue
		}
		t.Row(job.UUID, paused, job.Name, job.Summary,
			job.RetentionName, job.ScheduleName, job.TargetEndpoint, job.Agent)
	}
	t.Output(os.Stdout)
	return nil
}

func PauseUnpauseJob(p bool, u string) error {
	if p {
		err := PauseJob(uuid.Parse(u))
		if err != nil {
			return fmt.Errorf("ERROR: Could not pause job '%s': %s", u, err)
		}
	} else {
		err := UnpauseJob(uuid.Parse(u))
		if err != nil {
			return fmt.Errorf("ERROR: Could not unpause job '%s': %s", u, err)
		}
	}
	return nil
}

func processCreateJobRequest(cmd *cobra.Command, args []string) {

	// Validate Request
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "\nERROR: Unexpected arguments following command: %v\n", args)
		//FIXME  show help
		os.Exit(1)
	}

	// Invoke editor
	content := invokeEditor(`{
  "name"      : "Job Name",
  "summary"   : "a short description",

  "store"     : "uuid_of_store_to_use",
  "target"    : "uuid_of_target_to_use",
  "retention" : "uuid_of_retention_policy_to_use",
  "schedule"  : "uuid_of_schedule_to_use",

  "paused"    : false
}`)

	fmt.Println("Got the following content:\n\n", content)

	data, err := CreateJob(content)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nERROR: Could not fetch list of targets:\n", err)
		os.Exit(1)
	}

	// Print
	output, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nERROR: Could not render list of targets:\n", err)
		os.Exit(1)
	}

	fmt.Println(string(output[:]))

	return
}

func processEditJobRequest(cmd *cobra.Command, args []string) {

	if len(args) != 1 {
		fmt.Fprint(os.Stderr, "\nERROR: Requires a single UUID\n", args)
		//FIXME  show help
		os.Exit(1)
	}

	requested_UUID := uuid.Parse(args[0])

	original_data, err := GetJob(requested_UUID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nERROR: Could not show job:\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(original_data, "", "    ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nERROR: Could not render job:\n", err)
	}

	fmt.Println("Got the following original job:\n\n", string(data))

	// Invoke editor
	content := invokeEditor(string(data))

	fmt.Println("Got the following edited job:\n\n", content)

	update_data, err := UpdateJob(requested_UUID, content)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nERROR: Could not update jobs:\n", err)
		os.Exit(1)
	}
	// Print
	output, err := json.MarshalIndent(update_data, "", "    ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nERROR: Could not render job:\n", err)
		os.Exit(1)
	}

	fmt.Println(string(output[:]))

	return
}

func processDeleteJobRequest(cmd *cobra.Command, args []string) {

	if len(args) != 1 {
		fmt.Fprint(os.Stderr, "\nERROR: Requires a single UUID\n", args)
		//FIXME  show help
		os.Exit(1)
	}

	requested_UUID := uuid.Parse(args[0])

	err := DeleteJob(requested_UUID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nERROR: Could not delete job:\n", err)
		os.Exit(1)
	}

	// Print
	fmt.Println(requested_UUID, "deleted")

	return
}

func processRunJobRequest(cmd *cobra.Command, args []string) {

	if len(args) != 1 {
		fmt.Fprint(os.Stderr, "\nERROR: Requires a single UUID\n", args)
		//FIXME  show help
		os.Exit(1)
	}

	requested_UUID := uuid.Parse(args[0])

	// FIXME when owner can be passed in or otherwise fetched
	content := "{\"owner\":\"anon\"}"

	err := RunJob(requested_UUID, content)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nERROR: Could not run job:\n", err)
		os.Exit(1)
	}

	fmt.Println(requested_UUID, "scheduled")

	return
}

func processPausedJobRequest(cmd *cobra.Command, args []string) {

	if len(args) != 1 {
		fmt.Fprint(os.Stderr, "\nERROR: Requires a single UUID\n", args)
		//FIXME  show help
		os.Exit(1)
	}

	requested_UUID := uuid.Parse(args[0])

	paused, err := IsPausedJob(requested_UUID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nERROR: Could not pause job:\n", err)
		os.Exit(1)
	}

	if paused == true {
		fmt.Println("Job", requested_UUID, "is paused")
		os.Exit(0)
	} else {
		fmt.Println("Job", requested_UUID, "is not paused")
		os.Exit(1)
	}
	return
}
