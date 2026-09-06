package zenmux

import (
	"strconv"
	"strings"
)

func inferNativeRatioResolution(size string) (string, string, bool) {
	w, h, ok := parseSize(size)
	if !ok {
		return "", "", false
	}

	switch {
	case w == 256 && h == 256:
		return "1:1", "480p", true
	case w == 1280 && h == 720:
		return "16:9", "720p", true
	case w == 720 && h == 1280:
		return "9:16", "720p", true
	case w == 1920 && h == 1080:
		return "16:9", "1080p", true
	case w == 1080 && h == 1920:
		return "9:16", "1080p", true
	case w == 640 && h == 480:
		return "4:3", "480p", true
	case w == 480 && h == 640:
		return "3:4", "480p", true
	default:
		return "", "", false
	}
}

func parseSize(size string) (int, int, bool) {
	before, after, ok := strings.Cut(strings.TrimSpace(strings.ToLower(size)), "x")
	if !ok {
		return 0, 0, false
	}

	w, err := strconv.Atoi(strings.TrimSpace(before))
	if err != nil || w <= 0 {
		return 0, 0, false
	}

	h, err := strconv.Atoi(strings.TrimSpace(after))
	if err != nil || h <= 0 {
		return 0, 0, false
	}

	return w, h, true
}
