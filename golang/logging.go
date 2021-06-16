package csg

import (
	"fmt"
	"github.com/kataras/golog"
	"io"
	"runtime"
	"strings"
	"time"
)

var Log = newLogger()

type ScraperFormatter struct{}

// The name of the formatter.
func (s *ScraperFormatter) String() string {
	return "ScraperFormatter"
}

// Set any options and return a clone,
// generic. See `Logger.SetFormat`.
func (s *ScraperFormatter) Options(_ ...interface{}) golog.Formatter {
	// no customer options currently
	return s
}

// Writes the "log" to "dest" logger.
func (s *ScraperFormatter) Format(dest io.Writer, log *golog.Log) bool {
	timestamp := time.Now().Format(time.RFC1123)
	line := fmt.Sprintf("%s %s %s: %s%s", timestamp, golog.Levels[log.Level].Text(true), getCallingFunction(), log.Message, NEWLINE)
	if _, err := dest.Write([]byte(line)); err != nil {
		fmt.Printf("[FATAL] error in logger: %+v\n", err)
		return false
	}
	return true
}

// configure logging here
func newLogger() *golog.Logger {
	logger := golog.New()
	logger.RegisterFormatter(&ScraperFormatter{})
	logger.SetLevel("info")
	logger.SetFormat("ScraperFormatter")
	return logger
}

func getStackFrame(skipFrames int, skipFnNames []string) runtime.Frame {
	// We need the frame at index skipFrames+2, since we never want runtime.Callers and getFrame
	targetFrameIndex := skipFrames + 2

	for foundValidFrame := false; !foundValidFrame; {
		programCounters := make([]uintptr, targetFrameIndex+2)
		n := runtime.Callers(0, programCounters)

		var frame runtime.Frame
		if n > 0 {
			frames := runtime.CallersFrames(programCounters[:n])
			for more, frameIndex := true, 0; more && frameIndex <= targetFrameIndex; frameIndex++ {
				var frameCandidate runtime.Frame
				frameCandidate, more = frames.Next()
				if frameIndex == targetFrameIndex {
					frame = frameCandidate
				}
			}
		} else {
			break
		}

		foundValidFrame = true
		for _, skipFnName := range skipFnNames {
			if strings.Contains(frame.Function, skipFnName) {
				targetFrameIndex++
				foundValidFrame = false
				break
			}
		}

		if foundValidFrame {
			return frame
		}
	}

	return runtime.Frame{Function: "unknown"}
}

// returns the name of the function that called it :)
// skips any logging/goroutine stack frames
func getCallingFunction() string {
	skipFnNames := []string{"kataras", "golang.Run"}
	parts := strings.Split(getStackFrame(2, skipFnNames).Function, "/")
	return parts[len(parts)-1]
}
