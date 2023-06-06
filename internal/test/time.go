package test

import "time"

func MustParseTime(format string, value string) time.Time {
	t, err := time.Parse(format, value)
	if err != nil {
		panic(err)
	}
	return t
}
