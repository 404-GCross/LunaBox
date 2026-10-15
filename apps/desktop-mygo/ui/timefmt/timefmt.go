// Package timefmt 对应原版 frontend/src/utils/time.ts：时长与本地日期的显示格式（zh-CN）。
package timefmt

import (
	"fmt"
	"time"

	"lunabox/internal/appconf"
)

// Duration 对应 formatDuration：1小时5分钟、5分钟、30秒。
func Duration(seconds int) string {
	h, m, s := seconds/3600, seconds%3600/60, seconds%60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%d小时%d分钟", h, m)
	case h > 0:
		return fmt.Sprintf("%d小时", h)
	case m > 0:
		return fmt.Sprintf("%d分钟", m)
	}
	return fmt.Sprintf("%d秒", s)
}

// DurationChart 对应 formatDurationChart：图表坐标轴上的 5时 2分、0分。
func DurationChart(seconds int) string {
	total := max(0, seconds) / 60
	if h := total / 60; h > 0 {
		return fmt.Sprintf("%d时 %d分", h, total%60)
	}
	return fmt.Sprintf("%d分", total)
}

// Location 是配置里的时区（time_zone），没有配置或无法识别时用本地时区。
func Location(cfg *appconf.AppConfig) *time.Location {
	if cfg != nil && cfg.TimeZone != "" {
		if loc, err := time.LoadLocation(cfg.TimeZone); err == nil {
			return loc
		}
	}
	return time.Local
}

// Date 对应 formatLocalDate 在 zh-CN 下的结果：2026/09/11。
func Date(t time.Time, loc *time.Location) string { return t.In(loc).Format("2006/01/02") }

// DateTime 对应 formatLocalDateTime 在 zh-CN 下的结果：2026/09/11 13:05:09。
func DateTime(t time.Time, loc *time.Location) string { return t.In(loc).Format("2006/01/02 15:04:05") }

// ParseDate 解析后端返回的本地日期 YYYY-MM-DD（parseDateOnlyToLocalTimestamp）。
func ParseDate(s string, loc *time.Location) (time.Time, bool) {
	t, err := time.ParseInLocation("2006-01-02", s, loc)
	return t, err == nil
}
