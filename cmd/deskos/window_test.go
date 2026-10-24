package main

import (
	"testing"
	"time"
)

// 2026-10-24 is a Saturday.
var saturday = map[time.Weekday]bool{time.Saturday: true}

func at(hour, minute, second int) time.Time {
	return time.Date(2026, 10, 24, hour, minute, second, 0, time.UTC)
}

func TestWindowAtSupportsExactTimeAndWindow(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		days     map[time.Weekday]bool
		at       string
		grace    time.Duration
		start    string
		duration time.Duration
		want     bool
	}{
		{"exact at the minute", at(3, 0, 0), saturday, "03:00", 5 * time.Minute, "", 0, true},
		{"exact within grace", at(3, 4, 59), saturday, "03:00", 5 * time.Minute, "", 0, true},
		{"exact at the grace boundary", at(3, 5, 0), saturday, "03:00", 5 * time.Minute, "", 0, false},
		{"exact before the time", at(2, 59, 0), saturday, "03:00", 5 * time.Minute, "", 0, false},
		{"exact on another day", at(3, 0, 0), map[time.Weekday]bool{time.Sunday: true}, "03:00", 5 * time.Minute, "", 0, false},
		{"window before its start", at(2, 59, 0), saturday, "", 0, "03:00", 2 * time.Hour, false},
		{"window at its start", at(3, 0, 0), saturday, "", 0, "03:00", 2 * time.Hour, true},
		{"window inside", at(4, 59, 59), saturday, "", 0, "03:00", 2 * time.Hour, true},
		{"window at its end", at(5, 0, 0), saturday, "", 0, "03:00", 2 * time.Hour, false},
		{"window on another day", at(3, 30, 0), map[time.Weekday]bool{time.Sunday: true}, "", 0, "03:00", 2 * time.Hour, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := windowAt(tt.now, tt.days, tt.at, tt.grace, tt.start, tt.duration)
			if err != nil {
				t.Fatalf("windowAt: %v", err)
			}
			if got != tt.want {
				t.Fatalf("windowAt = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWindowAtRejectsIncompleteOrContradictoryArguments(t *testing.T) {
	tests := []struct {
		name     string
		at       string
		grace    time.Duration
		start    string
		duration time.Duration
	}{
		{"neither mode", "", 0, "", 0},
		{"both modes", "03:00", 5 * time.Minute, "03:00", 2 * time.Hour},
		{"bad exact time", "3:00", 5 * time.Minute, "", 0},
		{"negative grace", "03:00", -time.Minute, "", 0},
		{"window without duration", "", 0, "03:00", 0},
		{"window without start", "", 0, "", 2 * time.Hour},
		{"bad window start", "", 0, "3:00", 2 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := windowAt(at(3, 0, 0), saturday, tt.at, tt.grace, tt.start, tt.duration); err == nil {
				t.Fatal("windowAt accepted invalid arguments")
			}
		})
	}
}
