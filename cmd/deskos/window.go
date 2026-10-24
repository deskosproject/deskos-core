package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
	_ "time/tzdata"
)

// window is the ExecCondition of the scheduled apply service. It answers one
// question: is this moment inside the day and time range the plan allowed?
// A calendar timer that fires after suspend must not reboot a machine just
// because the wall clock moved past the window.
func window(args []string, _ io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("window", flag.ContinueOnError)
	fs.SetOutput(errOut)
	daysFlag := fs.String("days", "", "comma-separated weekdays, for example mon,wed")
	at := fs.String("at", "", "exact local time HH:MM")
	grace := fs.Duration("grace", 5*time.Minute, "how late an exact apply may still run")
	start := fs.String("start", "", "window start HH:MM")
	duration := fs.Duration("duration", 0, "window duration")
	tz := fs.String("timezone", "", "IANA time zone; the machine's local zone when empty")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	days, err := parseDays(*daysFlag)
	if err != nil {
		fmt.Fprintf(errOut, "deskos window: %v\n", err)
		return 2
	}
	loc := time.Local
	if *tz != "" {
		loc, err = time.LoadLocation(*tz)
		if err != nil {
			fmt.Fprintf(errOut, "deskos window: unknown time zone %q\n", *tz)
			return 2
		}
	}
	allowed, err := windowAt(time.Now().In(loc), days, *at, *grace, *start, *duration)
	if err != nil {
		fmt.Fprintf(errOut, "deskos window: %v\n", err)
		return 2
	}
	if !allowed {
		return 1
	}
	return 0
}

// windowAt answers the question for one instant, so the boundary rules are
// testable without waiting for a clock. `now` carries the local zone.
func windowAt(now time.Time, days map[time.Weekday]bool,
	at string, grace time.Duration, start string, duration time.Duration) (bool, error) {
	hasAt := at != ""
	hasWindow := start != "" || duration != 0
	if hasAt == hasWindow {
		return false, fmt.Errorf("set exactly one of --at or --start/--duration")
	}
	if hasAt {
		h, m, err := parseClock(at)
		if err != nil {
			return false, err
		}
		if grace < 0 {
			return false, fmt.Errorf("--grace must not be negative")
		}
		begin := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
		return days[now.Weekday()] && !now.Before(begin) && now.Before(begin.Add(grace)), nil
	}

	if start == "" || duration <= 0 {
		return false, fmt.Errorf("--start and a positive --duration are required together")
	}
	h, m, err := parseClock(start)
	if err != nil {
		return false, err
	}
	begin := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	end := begin.Add(duration)
	return days[now.Weekday()] && !now.Before(begin) && now.Before(end), nil
}

func parseClock(s string) (int, int, error) {
	if len(s) != 5 || s[2] != ':' {
		return 0, 0, fmt.Errorf("%q is not HH:MM", s)
	}
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, 0, fmt.Errorf("%q is not HH:MM", s)
	}
	return t.Hour(), t.Minute(), nil
}

func parseDays(s string) (map[time.Weekday]bool, error) {
	names := map[string]time.Weekday{
		"sun": time.Sunday, "sunday": time.Sunday,
		"mon": time.Monday, "monday": time.Monday,
		"tue": time.Tuesday, "tuesday": time.Tuesday,
		"wed": time.Wednesday, "wednesday": time.Wednesday,
		"thu": time.Thursday, "thursday": time.Thursday,
		"fri": time.Friday, "friday": time.Friday,
		"sat": time.Saturday, "saturday": time.Saturday,
	}
	out := map[time.Weekday]bool{}
	for _, part := range strings.Split(s, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		day, ok := names[name]
		if !ok {
			return nil, fmt.Errorf("unknown weekday %q", part)
		}
		out[day] = true
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--days is required")
	}
	return out, nil
}
