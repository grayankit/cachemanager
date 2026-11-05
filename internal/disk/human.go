package disk

import (
	"fmt"
	"strings"
)

func Format(n int64) string {
	if n < 0 {
		n = 0
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

func validName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '@' || r == '.' || r == '_' || r == '+' || r == '-':
		default:
			return false
		}
	}
	return true
}
