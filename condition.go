package collector

import "sync"

type BoolCondition struct {
	*sync.Cond
	Val bool
}

func NewBoolCondition() *BoolCondition {
	return &BoolCondition{
		Cond: sync.NewCond(new(sync.Mutex)),
	}
}

func (cond *BoolCondition) Broadcast() {
	cond.L.Lock()
	defer cond.L.Unlock()
	cond.Val = true
	cond.Cond.Broadcast()
}

func (cond *BoolCondition) Signal() {
	cond.L.Lock()
	defer cond.L.Unlock()
	cond.Val = true
	cond.Cond.Signal()
}

func (cond *BoolCondition) Unset() {
	cond.L.Lock()
	defer cond.L.Unlock()
	cond.Val = false
}

func (cond *BoolCondition) Wait() {
	cond.L.Lock()
	defer cond.L.Unlock()
	for !cond.Val {
		cond.Cond.Wait()
		if cond.Val {
			return
		}
	}
}

func (cond *BoolCondition) WaitAndUnset() {
	cond.L.Lock()
	defer cond.L.Unlock()
	for {
		if cond.Val {
			cond.Val = false
			return
		}
		cond.Cond.Wait()
		if cond.Val {
			cond.Val = false
			return
		}
	}
}
