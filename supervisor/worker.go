package supervisor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/starkandwayne/goutils/log"
	"github.com/starkandwayne/shield/agent"

	"github.com/pborman/uuid"
)

type UpdateOp int

const (
	STOPPED UpdateOp = iota
	FAILED
	OUTPUT
	RESTORE_KEY
)

type WorkerUpdate struct {
	Task      uuid.UUID
	Op        UpdateOp
	StoppedAt time.Time
	Output    string
}

type WorkerRequest struct {
	Operation      string `json:"operation"`
	TargetPlugin   string `json:"target_plugin"`
	TargetEndpoint string `json:"target_endpoint"`
	StorePlugin    string `json:"store_plugin"`
	StoreEndpoint  string `json:"store_endpoint"`
	RestoreKey     string `json:"restore_key"`
}

func worker(id uint, privateKeyFile string, work chan Task, updates chan WorkerUpdate) {
	config, err := agent.ConfigureSSHClient(privateKeyFile)
	if err != nil {
		log.Errorf("worker %d unable to read user key %s: %s; bailing out.\n",
			id, privateKeyFile, err)
		return
	}

	for t := range work {
		client := agent.NewClient(config)

		remote := t.Agent
		if remote == "" {
			updates <- WorkerUpdate{Task: t.UUID, Op: OUTPUT,
				Output: fmt.Sprintf("TASK FAILED!!  no remote agent specified for task %s\n", t.UUID)}
			updates <- WorkerUpdate{Task: t.UUID, Op: FAILED}
			continue
		}

		err = client.Dial(remote)
		if err != nil {
			updates <- WorkerUpdate{Task: t.UUID, Op: OUTPUT,
				Output: fmt.Sprintf("TASK FAILED!!  shield worker %d unable to connect to %s (%s)\n", id, remote, err)}
			updates <- WorkerUpdate{Task: t.UUID, Op: FAILED}
			continue
		}
		defer client.Close()

		// start a command and stream output
		final := make(chan string)
		partial := make(chan string)

		go func(out chan string, up chan WorkerUpdate, t Task, in chan string) {
			var buffer []string
			for {
				s, ok := <-in
				if !ok {
					break
				}

				switch s[0:2] {
				case "O:":
					buffer = append(buffer, s[2:])
				case "E:":
					up <- WorkerUpdate{
						Task:   t.UUID,
						Op:     OUTPUT,
						Output: s[2:] + "\n",
					}
				}
			}
			out <- strings.Join(buffer, "")
			close(out)
		}(final, updates, t, partial)

		command, err := json.Marshal(WorkerRequest{
			Operation:      t.Op.String(),
			TargetPlugin:   t.TargetPlugin,
			TargetEndpoint: t.TargetEndpoint,
			StorePlugin:    t.StorePlugin,
			StoreEndpoint:  t.StoreEndpoint,
			RestoreKey:     t.RestoreKey,
		})
		if err != nil {
			updates <- WorkerUpdate{Task: t.UUID, Op: OUTPUT,
				Output: fmt.Sprintf("TASK FAILED!! shield worker %d was unable to json encode the request bound for remote agent %s (%s)", id, remote, err),
			}
			updates <- WorkerUpdate{Task: t.UUID, Op: FAILED}
			continue
		}
		// exec the command
		err = client.Run(partial, string(command))
		if err != nil {
			updates <- WorkerUpdate{Task: t.UUID, Op: OUTPUT,
				Output: fmt.Sprintf("TASK FAILED!!  shield worker %d failed to execute the command against the remote agent %s (%s)\n", id, remote, err)}
			updates <- WorkerUpdate{Task: t.UUID, Op: FAILED}
		}

		out := <-final
		if t.Op == BACKUP {
			// parse JSON from standard output and get the restore key
			// (this might fail, we might not get a key, etc.)
			v := struct {
				Key string
			}{}

			buf := bytes.NewBufferString(out)
			dec := json.NewDecoder(buf)
			err := dec.Decode(&v)

			if err != nil {
				updates <- WorkerUpdate{Task: t.UUID, Op: OUTPUT,
					Output: fmt.Sprintf("WORKER FAILED!!  shield worker %d failed to parse JSON response from remote agent %s (%s)\n", id, remote, err)}

			} else {
				updates <- WorkerUpdate{
					Task:   t.UUID,
					Op:     RESTORE_KEY,
					Output: v.Key,
				}
			}
		}

		// signal to the supervisor that we finished
		updates <- WorkerUpdate{
			Task:      t.UUID,
			Op:        STOPPED,
			StoppedAt: time.Now(),
		}
	}
}

func (s *Supervisor) SpawnWorker() {
	s.nextWorker += 1
	go worker(s.nextWorker, s.PrivateKeyFile, s.workers, s.updates)
}
