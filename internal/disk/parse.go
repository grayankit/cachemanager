package disk

import (
	"strconv"
	"strings"
)

type FS struct {
	Mount      string `json:"mount"`
	Size       int64  `json:"size"`
	Used       int64  `json:"used"`
	Avail      int64  `json:"avail"`
	UsePercent int    `json:"usePercent"`
	Label      string `json:"label"`
	FreeLabel  string `json:"freeLabel"`
}

func (f FS) withLabels() FS {
	f.Label = Format(f.Used) + " used of " + Format(f.Size) + ", " + Format(f.Avail) + " free"
	f.FreeLabel = Format(f.Avail) + " free"
	return f
}

func ParseDF(out string) (FS, error) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return FS{}, errf("unexpected df output")
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 6 {
		return FS{}, errf("unexpected df line")
	}
	size, err1 := strconv.ParseInt(fields[1], 10, 64)
	used, err2 := strconv.ParseInt(fields[2], 10, 64)
	avail, err3 := strconv.ParseInt(fields[3], 10, 64)
	pct, err4 := strconv.Atoi(strings.TrimSuffix(fields[4], "%"))
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return FS{}, errf("could not parse df numbers")
	}
	return FS{
		Mount:      strings.Join(fields[5:], " "),
		Size:       size,
		Used:       used,
		Avail:      avail,
		UsePercent: pct,
	}.withLabels(), nil
}

func ParseDU(out string) map[string]int64 {
	sizes := map[string]int64{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		sizeStr, path, ok := strings.Cut(line, "\t")
		if !ok {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			sizeStr = fields[0]
			path = strings.Join(fields[1:], " ")
		}
		n, err := strconv.ParseInt(strings.TrimSpace(sizeStr), 10, 64)
		if err != nil || path == "" {
			continue
		}
		sizes[path] = n
	}
	return sizes
}

func ParsePaccache(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "==>") {
			continue
		}
		if strings.Contains(line, "/") || strings.Contains(line, "..") {
			continue
		}
		if strings.Contains(line, ".pkg.tar.") {
			names = append(names, line)
		}
	}
	return names
}
