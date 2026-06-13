package recorder

import (
	"strings"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	rtspformat "github.com/bluenviron/gortsplib/v5/pkg/format"

	"github.com/google/uuid"

	"github.com/bluenviron/mediamtx/internal/conf"
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/recordstore"
	"github.com/bluenviron/mediamtx/internal/stream"
	"github.com/bluenviron/mediamtx/internal/unit"
)

type recorderInstance struct {
	pathFormat        string
	format            conf.RecordFormat
	partDuration      time.Duration
	maxPartSize       conf.StringSize
	segmentDuration   time.Duration
	motion            bool
	motionThreshold   int
	motionMinPixels   int
	pathName          string
	stream            *stream.Stream
	onSegmentCreate   OnSegmentCreateFunc
	onSegmentComplete OnSegmentCompleteFunc
	parent            logger.Writer

	streamID     uuid.UUID
	pathFormat2  string
	format2      format
	skip         bool
	reader       *stream.Reader
	motionReader *stream.Reader

	terminate chan struct{}
	done      chan struct{}
}

// Log implements logger.Writer.
func (ri *recorderInstance) Log(level logger.Level, format string, args ...any) {
	ri.parent.Log(level, format, args...)
}

func (ri *recorderInstance) initialize() {
	ri.streamID = uuid.New()
	ri.pathFormat2 = ri.pathFormat
	ri.pathFormat2 = recordstore.PathAddExtension(
		strings.ReplaceAll(ri.pathFormat2, "%path", ri.pathName),
		ri.format,
	)
	ri.reader = &stream.Reader{
		SkipOutboundBytes: true,
		Parent:            ri,
	}

	ri.terminate = make(chan struct{})
	ri.done = make(chan struct{})

	switch ri.format {
	case conf.RecordFormatMPEGTS:
		ri.format2 = &formatMPEGTS{
			ri: ri,
		}
		ok := ri.format2.initialize()
		ri.skip = !ok

	default:
		ri.format2 = &formatFMP4{
			ri: ri,
		}
		ok := ri.format2.initialize()
		ri.skip = !ok
	}

	if !ri.skip {
		ri.stream.AddReader(ri.reader)
		if ri.motion {
			ri.initializeMotionReader()
		}
	}

	go ri.run()
}

func (ri *recorderInstance) close() {
	close(ri.terminate)
	<-ri.done
}

func (ri *recorderInstance) run() {
	defer close(ri.done)

	if !ri.skip {
		select {
		case err := <-ri.reader.Error():
			ri.Log(logger.Error, err.Error())

		case <-ri.terminate:
		}

		if ri.motionReader != nil {
			ri.stream.RemoveReader(ri.motionReader)
		}

		ri.stream.RemoveReader(ri.reader)
	} else {
		<-ri.terminate
	}

	ri.format2.close()
}

func (ri *recorderInstance) initializeMotionReader() {
	var mjpegMedia *description.Media
	var mjpegFormat *rtspformat.MJPEG
	mjpegMedia = ri.stream.Desc.FindFormat(&mjpegFormat)
	if mjpegFormat == nil {
		ri.Log(logger.Warn, "motion-based recording requires a MJPEG video track; motion markers won't be created")
		return
	}

	detector := &motionDetector{
		threshold: uint8(ri.motionThreshold),
		minPixels: ri.motionMinPixels,
	}

	ri.motionReader = &stream.Reader{
		SkipOutboundBytes: true,
		Parent:            ri,
	}
	ri.motionReader.OnData(mjpegMedia, mjpegFormat, func(u *unit.Unit) error {
		if u.NilPayload() {
			return nil
		}

		motion, err := detector.detectJPEG(u.Payload.(unit.PayloadMJPEG))
		if err != nil {
			return nil
		}

		if motion {
			ri.parent.(*Recorder).onMotionDetected()
		}

		return nil
	})
	ri.stream.AddReader(ri.motionReader)
}
