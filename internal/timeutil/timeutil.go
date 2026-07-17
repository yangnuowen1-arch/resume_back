package timeutil

import "time"

var Shanghai = time.FixedZone("Asia/Shanghai", 8*60*60)

func Now() time.Time {
	return time.Now().In(Shanghai)
}

func FormatTimestamp(t time.Time) string {
	return t.In(Shanghai).Format(time.RFC3339)
}
