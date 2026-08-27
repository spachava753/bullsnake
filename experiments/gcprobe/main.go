package main

import (
	"fmt"
	"runtime"
	"time"
	"weak"
)

const observationTimeout = 2 * time.Second

type value interface {
	isValue()
}

// object approximates a heap-allocated Python value with GC-visible references.
type object struct {
	name    string
	refs    []value
	payload [64]byte
}

func (*object) isValue() {}

type cleanupNotice struct {
	name string
	done chan<- string
}

func sendCleanup(notice cleanupNotice) {
	notice.done <- notice.name
}

type weakNotice struct {
	target weak.Pointer[object]
	done   chan<- bool
}

func inspectWeakFromCleanup(notice *weakNotice) {
	notice.done <- notice.target.Value() == nil
}

type blockingNotice struct {
	started  chan struct{}
	release  chan struct{}
	finished chan struct{}
}

func blockInCleanup(notice *blockingNotice) {
	close(notice.started)
	<-notice.release
	close(notice.finished)
}

func main() {
	fmt.Printf("Bullsnake Go GC probe\n")
	fmt.Printf("runtime: %s %s/%s\n\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	probeOrdinaryCycle()
	probeWeakNotification()
	probeCleanupScheduling()
	probeFinalizerResurrection()
	probeFinalizerCycle()

	fmt.Println("\nThese are bounded observations from one run.")
	fmt.Println("Go's documented guarantees, not a successful observation, define what Bullsnake may rely on.")
}

func probeOrdinaryCycle() {
	cleaned := make(chan string, 2)
	left, right := makeCollectableCycle(cleaned)
	seen := make(map[string]bool)
	weakPointersCleared := false

	ok, rounds := driveGC(observationTimeout, func() bool {
		drainStrings(cleaned, seen)
		weakPointersCleared = left.Value() == nil && right.Value() == nil
		return weakPointersCleared && len(seen) == 2
	})

	if ok {
		report("observed", "ordinary cycle", fmt.Sprintf(
			"both weak pointers cleared and both cleanups ran after %d forced collections", rounds,
		))
		return
	}

	report("inconclusive", "ordinary cycle", fmt.Sprintf(
		"weak pointers cleared=%t, cleanups=%d/2 after %d forced collections",
		weakPointersCleared, len(seen), rounds,
	))
}

//go:noinline
func makeCollectableCycle(cleaned chan<- string) (weak.Pointer[object], weak.Pointer[object]) {
	left := &object{name: "left"}
	right := &object{name: "right"}
	left.refs = []value{right}
	right.refs = []value{left}

	runtime.AddCleanup(left, sendCleanup, cleanupNotice{name: left.name, done: cleaned})
	runtime.AddCleanup(right, sendCleanup, cleanupNotice{name: right.name, done: cleaned})

	leftWeak := weak.Make(left)
	rightWeak := weak.Make(right)
	runtime.KeepAlive(left)
	runtime.KeepAlive(right)
	return leftWeak, rightWeak
}

func probeWeakNotification() {
	result := make(chan bool, 1)
	makeWeakNotification(result)

	var targetWasGone bool
	ok, rounds := driveGC(observationTimeout, func() bool {
		select {
		case targetWasGone = <-result:
			return true
		default:
			return false
		}
	})

	if ok {
		report("observed", "weak notification", fmt.Sprintf(
			"cleanup ran after %d forced collections and weak target was nil=%t", rounds, targetWasGone,
		))
		return
	}

	report("inconclusive", "weak notification", fmt.Sprintf(
		"cleanup did not run within %d forced collections", rounds,
	))
}

//go:noinline
func makeWeakNotification(result chan<- bool) {
	target := &object{name: "weak-target"}
	notice := &weakNotice{target: weak.Make(target), done: result}
	runtime.AddCleanup(target, inspectWeakFromCleanup, notice)
	runtime.KeepAlive(target)
}

func probeCleanupScheduling() {
	notice := &blockingNotice{
		started:  make(chan struct{}),
		release:  make(chan struct{}),
		finished: make(chan struct{}),
	}
	makeBlockingCleanup(notice)

	started := false
	ok, rounds := driveGC(observationTimeout, func() bool {
		if started {
			return true
		}
		select {
		case <-notice.started:
			started = true
			return true
		default:
			return false
		}
	})

	if !ok {
		report("inconclusive", "cleanup scheduling", fmt.Sprintf(
			"cleanup did not start within %d forced collections", rounds,
		))
		close(notice.release)
		return
	}

	finishedBeforeRelease := channelClosed(notice.finished)
	close(notice.release)
	finishedAfterRelease := waitForClose(notice.finished, observationTimeout)

	if !finishedBeforeRelease && finishedAfterRelease {
		report("observed", "cleanup scheduling",
			"a cleanup ran concurrently and runtime.GC returned before that cleanup completed")
		return
	}

	report("inconclusive", "cleanup scheduling", fmt.Sprintf(
		"finished before release=%t, finished after release=%t",
		finishedBeforeRelease, finishedAfterRelease,
	))
}

//go:noinline
func makeBlockingCleanup(notice *blockingNotice) {
	target := &object{name: "blocking-cleanup"}
	runtime.AddCleanup(target, blockInCleanup, notice)
	runtime.KeepAlive(target)
}

func probeFinalizerResurrection() {
	finalized := make(chan *object, 1)
	cleaned := make(chan string, 1)
	oldWeak := makeResurrectable(finalized, cleaned)

	var resurrected *object
	ok, rounds := driveGC(observationTimeout, func() bool {
		select {
		case resurrected = <-finalized:
			return true
		default:
			return false
		}
	})

	if !ok {
		report("inconclusive", "finalizer resurrection", fmt.Sprintf(
			"acyclic finalizer did not run within %d forced collections", rounds,
		))
		return
	}

	name := resurrected.name
	oldWeakCleared := oldWeak.Value() == nil
	newWeak := weak.Make(resurrected)
	newWeakLive := newWeak.Value() != nil
	weakIdentityChanged := oldWeak != newWeak
	runtime.KeepAlive(resurrected)
	resurrected = nil

	cleanupRan := false
	cleanupOK, cleanupRounds := driveGC(observationTimeout, func() bool {
		select {
		case <-cleaned:
			cleanupRan = true
		default:
		}
		return cleanupRan && newWeak.Value() == nil
	})

	report("observed", "finalizer resurrection", fmt.Sprintf(
		"received %q after %d collections; old weak cleared=%t, new weak live=%t, weak identity changed=%t",
		name, rounds, oldWeakCleared, newWeakLive, weakIdentityChanged,
	))

	if cleanupOK {
		report("observed", "post-finalizer cleanup", fmt.Sprintf(
			"cleanup ran after resurrection ended and %d more forced collections", cleanupRounds,
		))
		return
	}

	report("inconclusive", "post-finalizer cleanup", fmt.Sprintf(
		"cleanup ran=%t after %d more forced collections", cleanupRan, cleanupRounds,
	))
}

//go:noinline
func makeResurrectable(finalized chan<- *object, cleaned chan<- string) weak.Pointer[object] {
	target := &object{name: "resurrected-target"}
	originalWeak := weak.Make(target)
	runtime.AddCleanup(target, sendCleanup, cleanupNotice{name: target.name, done: cleaned})
	runtime.SetFinalizer(target, func(obj *object) {
		finalized <- obj
	})
	runtime.KeepAlive(target)
	return originalWeak
}

func probeFinalizerCycle() {
	finalized := make(chan string, 1)
	target := makeFinalizerCycle(finalized)

	var name string
	ok, rounds := driveGC(750*time.Millisecond, func() bool {
		select {
		case name = <-finalized:
			return true
		default:
			return false
		}
	})

	if ok {
		report("observed", "finalizer in cycle", fmt.Sprintf(
			"finalizer for %q ran after %d forced collections; Go still provides no guarantee", name, rounds,
		))
		return
	}

	report("not observed", "finalizer in cycle", fmt.Sprintf(
		"finalizer did not run after %d forced collections and weak target remained live=%t",
		rounds, target.Value() != nil,
	))
}

//go:noinline
func makeFinalizerCycle(finalized chan<- string) weak.Pointer[object] {
	target := &object{name: "self-cycle"}
	target.refs = []value{target}
	runtime.SetFinalizer(target, func(obj *object) {
		finalized <- obj.name
	})
	result := weak.Make(target)
	runtime.KeepAlive(target)
	return result
}

func driveGC(timeout time.Duration, ready func() bool) (bool, int) {
	deadline := time.Now().Add(timeout)
	rounds := 0
	for {
		if ready() {
			return true, rounds
		}
		if time.Now().After(deadline) {
			return false, rounds
		}

		rounds++
		runtime.GC()
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
	}
}

func drainStrings(ch <-chan string, seen map[string]bool) {
	for {
		select {
		case value := <-ch:
			seen[value] = true
		default:
			return
		}
	}
}

func channelClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func waitForClose(ch <-chan struct{}, timeout time.Duration) bool {
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

func report(status, experiment, detail string) {
	fmt.Printf("[%s] %s: %s\n", status, experiment, detail)
}
