package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"integration/app/logging"
	"integration/app/tree"
	"sync"
	"time"
)

type Job struct {
	DataverseKey  string
	PersistentId  string
	WritableNodes map[string]tree.Node
	StreamType    string
	Streams       map[string]map[string]interface{}
	StreamParams  map[string]string
}

var Stop = make(chan struct{})
var Wait = sync.WaitGroup{}

var lockMaxDuration = time.Hour * 24

func lock(persistentId string) bool {
	ok := rdb.SetNX(context.Background(), "lock: "+persistentId, true, lockMaxDuration)
	return ok.Val()
}

func unlock(persistentId string) {
	rdb.Del(context.Background(), "lock: "+persistentId)
}

func AddJob(job Job) error {
	if len(job.WritableNodes) == 0 {
		return nil
	}
	err := addJob(job, true)
	if err == nil {
		logging.Logger.Println("job added for " + job.PersistentId)
	}
	return err
}

func addJob(job Job, requireLock bool) error {
	if len(job.WritableNodes) == 0 {
		return nil
	}
	if requireLock && !lock(job.PersistentId) {
		return fmt.Errorf("Job for this dataverse is already in progress")
	}
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	cmd := rdb.LPush(context.Background(), "jobs", string(b))
	return cmd.Err()
}

func popJob() (Job, bool) {
	cmd := rdb.RPop(context.Background(), "jobs")
	err := cmd.Err()
	if err != nil {
		return Job{}, false
	}
	v := cmd.Val()
	job := Job{}
	err = json.Unmarshal([]byte(v), &job)
	if err != nil {
		logging.Logger.Println("failed to unmarshall a job:", err)
		return job, false
	}
	return job, true
}

func ProcessJobs() {
	Wait.Add(1)
	defer logging.Logger.Println("worker exited grecefully")
	defer Wait.Done()
	for {
		select {
		case <-Stop:
			return
		case <-time.After(10 * time.Second):
		}
		job, ok := popJob()
		if ok {
			persistentId := job.PersistentId
			job, err := doWork(job)
			if err != nil {
				logging.Logger.Println("job failed:", persistentId, err)
			}
			if err == nil && len(job.WritableNodes) > 0 {
				err = addJob(job, false)
				if err != nil {
					logging.Logger.Println("re-adding job failed:", persistentId, err)
				}
			} else {
				unlock(persistentId)
			}
		}
	}
}
