package pkg

import (
	"strconv"
	"time"
)

const TimeFormat = "2006-01-02 15:04:05"
const TimeFormatDay = "2006-01-02"

const Day = time.Hour * 24

const UnixHour int64 = 3600

const UnixDay = UnixHour * 24

const UnixWeek = UnixDay * 7

func Local() time.Time {
	return time.Now().Local()
}

func UTC() time.Time {
	return time.Now().UTC()
}

func UnixTime() int64 {
	return UTC().Unix()
}

// TimeStamp 返回截断到秒的 UTC 时间。
func TimeStamp() time.Time {
	return UTC().Truncate(time.Second)
}

func TimePointer() *time.Time {
	return new(TimeStamp())
}

func Seconds(s int) time.Duration {
	return time.Duration(s) * time.Second
}

func Yesterday() time.Time {
	return UTC().Add(-24 * time.Hour)
}
func Sec2Time(timeStamp string) time.Time {
	i, err := strconv.ParseInt(timeStamp, 10, 64)
	if err != nil {
		panic(err)
	}
	return time.Unix(i, 0)
}
func Fmt2HMS(time time.Time) string {
	return time.Format("2006-01-02 15:04:05")
}
func NowTimeStr() string {
	return time.Now().Local().Format("2006-01-02 15:04:05")
}
func NowTimeStr2() string {
	return time.Now().Local().Format("20060102150405")
}
func Fmt2Day(time time.Time) string {
	return time.Format("2006-01-02")
}
func Fmt2Month(time time.Time) string {
	return time.Format("2006-01")
}
