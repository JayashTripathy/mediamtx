// Package recorder contains the recorder.
package recorder

import (
	"os"
	"sync"
	"time"

	"github.com/bluenviron/mediamtx/internal/conf"
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/stream"
)

const (
	ntpDriftTolerance = 5 * time.Second
)

// OnSegmentCreateFunc is the prototype of the function passed as OnSegmentCreate
type OnSegmentCreateFunc = func(path string)

// OnSegmentCompleteFunc is the prototype of the function passed as OnSegmentComplete
type OnSegmentCompleteFunc = func(path string, duration time.Duration)

// MotionMarkerSuffix is appended to segments that contain detected motion.
const MotionMarkerSuffix = ".motion"

// NoMotionMarkerSuffix is appended to segments that do not contain detected motion.
const NoMotionMarkerSuffix = ".nomotion"

// Recorder writes recordings to disk.
type Recorder struct {
	PathFormat        string
	Format            conf.RecordFormat
	PartDuration      time.Duration
	MaxPartSize       conf.StringSize
	SegmentDuration   time.Duration
	Motion            bool
	MotionThreshold   int
	MotionMinPixels   int
	PathName          string
	Stream            *stream.Stream
	OnSegmentCreate   OnSegmentCreateFunc
	OnSegmentComplete OnSegmentCompleteFunc
	Parent            logger.Writer

	restartPause time.Duration

	motionMutex        sync.Mutex
	currentSegmentPath string
	currentHasMotion   bool
	segmentHasMotion   map[string]bool

	wrappedOnSegmentCreate   OnSegmentCreateFunc
	wrappedOnSegmentComplete OnSegmentCompleteFunc

	currentInstance *recorderInstance

	terminate chan struct{}
	done      chan struct{}
}

// Initialize initializes Recorder.
func (r *Recorder) Initialize() {
	if r.OnSegmentCreate == nil {
		r.OnSegmentCreate = func(string) {
		}
	}
	if r.OnSegmentComplete == nil {
		r.OnSegmentComplete = func(string, time.Duration) {
		}
	}
	if r.restartPause == 0 {
		r.restartPause = 2 * time.Second
	}
	if r.MotionThreshold == 0 {
		r.MotionThreshold = 25
	}
	if r.MotionMinPixels == 0 {
		r.MotionMinPixels = 500
	}
	r.segmentHasMotion = make(map[string]bool)

	r.terminate = make(chan struct{})
	r.done = make(chan struct{})

	r.wrappedOnSegmentCreate = r.OnSegmentCreate
	r.wrappedOnSegmentComplete = r.OnSegmentComplete
	if r.Motion {
		r.wrappedOnSegmentCreate = func(path string) {
			r.onMotionSegmentCreate(path)
			r.OnSegmentCreate(path)
		}
		r.wrappedOnSegmentComplete = func(path string, duration time.Duration) {
			r.onMotionSegmentComplete(path)
			r.OnSegmentComplete(path, duration)
		}
	}

	r.currentInstance = &recorderInstance{
		pathFormat:        r.PathFormat,
		format:            r.Format,
		partDuration:      r.PartDuration,
		maxPartSize:       r.MaxPartSize,
		segmentDuration:   r.SegmentDuration,
		motion:            r.Motion,
		motionThreshold:   r.MotionThreshold,
		motionMinPixels:   r.MotionMinPixels,
		pathName:          r.PathName,
		stream:            r.Stream,
		onSegmentCreate:   r.wrappedOnSegmentCreate,
		onSegmentComplete: r.wrappedOnSegmentComplete,
		parent:            r,
	}
	r.currentInstance.initialize()

	go r.run()
}

// Log implements logger.Writer.
func (r *Recorder) Log(level logger.Level, format string, args ...any) {
	r.Parent.Log(level, "[recorder] "+format, args...)
}

// Close closes the agent.
func (r *Recorder) Close() {
	r.Log(logger.Info, "recording stopped")
	close(r.terminate)
	<-r.done
}

func (r *Recorder) run() {
	defer close(r.done)

	for {
		select {
		case <-r.currentInstance.done:
			r.currentInstance.close()
		case <-r.terminate:
			r.currentInstance.close()
			return
		}

		select {
		case <-time.After(r.restartPause):
		case <-r.terminate:
			return
		}

		r.currentInstance = &recorderInstance{
			pathFormat:        r.PathFormat,
			format:            r.Format,
			partDuration:      r.PartDuration,
			maxPartSize:       r.MaxPartSize,
			segmentDuration:   r.SegmentDuration,
			motion:            r.Motion,
			motionThreshold:   r.MotionThreshold,
			motionMinPixels:   r.MotionMinPixels,
			pathName:          r.PathName,
			stream:            r.Stream,
			onSegmentCreate:   r.wrappedOnSegmentCreate,
			onSegmentComplete: r.wrappedOnSegmentComplete,
			parent:            r,
		}
		r.currentInstance.initialize()
	}
}

func (r *Recorder) onMotionSegmentCreate(path string) {
	r.motionMutex.Lock()
	defer r.motionMutex.Unlock()

	r.currentSegmentPath = path
	r.segmentHasMotion[path] = r.currentHasMotion
}

func (r *Recorder) onMotionDetected() {
	r.motionMutex.Lock()
	defer r.motionMutex.Unlock()

	r.currentHasMotion = true
	if r.currentSegmentPath != "" {
		r.segmentHasMotion[r.currentSegmentPath] = true
	}
}

func (r *Recorder) onMotionSegmentComplete(path string) {
	r.motionMutex.Lock()
	hasMotion := r.segmentHasMotion[path]
	delete(r.segmentHasMotion, path)
	if r.currentSegmentPath == path {
		r.currentSegmentPath = ""
		r.currentHasMotion = false
	}
	r.motionMutex.Unlock()

	if hasMotion {
		_ = os.Remove(path + NoMotionMarkerSuffix)
		_ = os.WriteFile(path+MotionMarkerSuffix, nil, 0o644)
	} else {
		_ = os.Remove(path + MotionMarkerSuffix)
		_ = os.WriteFile(path+NoMotionMarkerSuffix, nil, 0o644)
	}
}
