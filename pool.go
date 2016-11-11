package collector

import (
	"sync"

	"github.com/antongulenko/golib"
)

type CollectorTask func() error

type CollectorTaskPolicyType int

var CollectorTaskPolicy = CollectorTasksUntilError

const (
	CollectorTasksSequential = CollectorTaskPolicyType(0)
	CollectorTasksParallel   = CollectorTaskPolicyType(1)
	CollectorTasksUntilError = CollectorTaskPolicyType(2)
)

type CollectorTasks []CollectorTask

func (pool CollectorTasks) Run() error {
	switch CollectorTaskPolicy {
	case CollectorTasksSequential:
		return pool.RunSequential()
	case CollectorTasksParallel:
		return pool.RunParallel()
	default:
		fallthrough
	case CollectorTasksUntilError:
		return pool.RunUntilError()
	}
}

func (pool CollectorTasks) RunParallel() error {
	var wg sync.WaitGroup
	var errors golib.MultiError
	var errorsLock sync.Mutex
	wg.Add(len(pool))
	for _, task := range pool {
		go func(task CollectorTask) {
			defer wg.Done()
			err := task()
			errorsLock.Lock()
			defer errorsLock.Unlock()
			errors.Add(err)
		}(task)
	}
	wg.Wait()
	return errors.NilOrError()
}

func (pool CollectorTasks) RunSequential() error {
	var errors golib.MultiError
	for _, task := range pool {
		err := task()
		errors.Add(err)
	}
	return errors.NilOrError()
}

func (pool CollectorTasks) RunUntilError() error {
	for _, task := range pool {
		if err := task(); err != nil {
			return err
		}
	}
	return nil
}
