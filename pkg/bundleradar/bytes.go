package bundleradar

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseBytes converts human-readable size strings like "200KB", "1.5MB", "1024B", "-10KB" into signed int64 bytes.
func ParseBytes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty byte string")
	}

	negative := false
	if strings.HasPrefix(s, "-") {
		negative = true
		s = strings.TrimSpace(strings.TrimPrefix(s, "-"))
	} else if strings.HasPrefix(s, "+") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "+"))
	}

	upper := strings.ToUpper(s)
	multiplier := int64(1)

	switch {
	case strings.HasSuffix(upper, "KB") || strings.HasSuffix(upper, "K"):
		multiplier = 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "KB"), "kb")
		s = strings.TrimSuffix(strings.TrimSuffix(s, "Kb"), "k")
		s = strings.TrimSuffix(s, "K")
	case strings.HasSuffix(upper, "MB") || strings.HasSuffix(upper, "M"):
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "MB"), "mb")
		s = strings.TrimSuffix(strings.TrimSuffix(s, "Mb"), "m")
		s = strings.TrimSuffix(s, "M")
	case strings.HasSuffix(upper, "GB") || strings.HasSuffix(upper, "G"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "GB"), "gb")
		s = strings.TrimSuffix(s, "G")
	case strings.HasSuffix(upper, "B"):
		multiplier = 1
		s = strings.TrimSuffix(strings.TrimSuffix(s, "B"), "b")
	}

	s = strings.TrimSpace(s)
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q: %w", s, err)
	}

	bytesFloat := val * float64(multiplier)
	if bytesFloat > float64(math.MaxInt64) {
		return 0, fmt.Errorf("byte value exceeds int64 max")
	}

	result := int64(bytesFloat)
	if negative {
		result = -result
	}
	return result, nil
}
