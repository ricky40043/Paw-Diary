package main

// ============================================================================
// Configuration
// ============================================================================

import (
	"os"
	"strconv"
)

var (
	// AnalysisDurationSeconds - How many seconds of video to analyze from the start (Phase 2)
	AnalysisDurationSeconds = 15

	// AnalysisInterval - Seconds between frames (e.g., 2.0 = 1 frame every 2 seconds)
	AnalysisInterval = 2.0

	// SegmentDurationSeconds - Duration of each analysis segment in seconds
	SegmentDurationSeconds = 3

	// MaxAIImages - Maximum number of images to send to AI for analysis per video
	MaxAIImages = 10
)

func LoadConfig() {
	if val := os.Getenv("ANALYSIS_DURATION_SECONDS"); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			AnalysisDurationSeconds = i
		}
	}

	if val := os.Getenv("ANALYSIS_INTERVAL"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			AnalysisInterval = f
		}
	}

	if val := os.Getenv("SEGMENT_DURATION_SECONDS"); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			SegmentDurationSeconds = i
		}
	}

	if val := os.Getenv("MAX_AI_IMAGES"); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			MaxAIImages = i
		}
	}
}
