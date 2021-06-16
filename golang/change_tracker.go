package csg

import (
	"fmt"
	"sync"
	"time"
)

type ChangeTracker struct {
	locker         map[string]bool
	errorCount     map[string]int
	apiLastTime    map[string]int64
	apiLastStatus  map[string]Status
	lastScrapeTime map[string]int64
	mutex          *sync.Mutex
}

func NewChangeTracker(names []string) *ChangeTracker {
	changeTracker := new(ChangeTracker)
	changeTracker.lastScrapeTime = make(map[string]int64)
	changeTracker.apiLastTime = make(map[string]int64)
	changeTracker.apiLastStatus = make(map[string]Status)
	changeTracker.locker = make(map[string]bool)
	changeTracker.errorCount = make(map[string]int)
	changeTracker.mutex = &sync.Mutex{}

	for _, name := range names {
		changeTracker.apiLastTime[name] = 0
		changeTracker.apiLastStatus[name] = StatusUnknown
		changeTracker.locker[name] = false
		changeTracker.errorCount[name] = 0
	}

	return changeTracker
}

func (t *ChangeTracker) Error(name string, err error) int {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.lastScrapeTime[name] = time.Now().Unix()

	prevErrorCount, ok := t.errorCount[name]
	if !ok {
		panic(fmt.Errorf("Name not found in change tracker object: %s", name))
	}

	t.errorCount[name] = prevErrorCount + 1

	return t.errorCount[name]
}

func (t *ChangeTracker) UpdateAndUnlock(name string, status Status) (bool, bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	_, ok := t.locker[name]
	if !ok {
		return false, false
	}

	t.locker[name] = false

	prevStatus, ok := t.apiLastStatus[name]
	if !ok {
		panic(fmt.Errorf("Name not found in change tracker object: %s", name))
	}

	t.errorCount[name] = 0

	currentTimestamp := time.Now().Unix()
	t.lastScrapeTime[name] = currentTimestamp

	if status == StatusUnknown {
		return false, false
	}

	if prevStatus != status {
		t.apiLastStatus[name] = status
		t.apiLastTime[name] = currentTimestamp

		if prevStatus == StatusUnknown {
			return true, false
		}

		return true, true
	}

	if (currentTimestamp - t.apiLastTime[name]) >= config.ApiInterval {
		t.apiLastTime[name] = currentTimestamp
		return true, false
	}

	return false, false
}

func (t *ChangeTracker) Lock(name string) bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	locked, ok := t.locker[name]
	if !ok || locked {
		return false
	}

	t.locker[name] = true

	return true
}

func (t *ChangeTracker) LastScrape(name string) int64 {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	lastScrapeTime, ok := t.lastScrapeTime[name]
	if ok {
		return lastScrapeTime
	}

	return 0
}
