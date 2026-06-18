package scheduler

import (
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	"venue-booking-admin/internal/models"
)

type ConflictInfo struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type ValidateResult struct {
	OK        bool           `json:"ok"`
	Conflicts []ConflictInfo `json:"conflicts,omitempty"`
}

func CheckConflicts(db *gorm.DB, s *models.Schedule, excludeID uint) (ValidateResult, []ConflictInfo) {
	var conflicts []ConflictInfo

	if s.EndHour <= s.StartHour {
		conflicts = append(conflicts, ConflictInfo{
			Type:    "time_invalid",
			Message: "结束时间须晚于开始时间",
		})
	}

	var venue models.Venue
	if err := db.First(&venue, s.VenueID).Error; err != nil {
		conflicts = append(conflicts, ConflictInfo{
			Type:    "venue_not_found",
			Message: "场馆不存在",
		})
	} else {
		if venue.Status != "open" {
			conflicts = append(conflicts, ConflictInfo{
				Type:    "venue_closed",
				Message: fmt.Sprintf("场馆当前状态为%s，不可排课", venue.Status),
			})
		}
		if s.StartHour < venue.OpenHour || s.EndHour > venue.CloseHour {
			conflicts = append(conflicts, ConflictInfo{
				Type:    "venue_outside_hours",
				Message: fmt.Sprintf("排课时段超出场馆开放时间(%d:00-%d:00)", venue.OpenHour, venue.CloseHour),
			})
		}
	}

	var coach models.Coach
	if err := db.First(&coach, s.CoachID).Error; err != nil {
		conflicts = append(conflicts, ConflictInfo{
			Type:    "coach_not_found",
			Message: "教练不存在",
		})
	} else {
		if coach.Status != "active" {
			conflicts = append(conflicts, ConflictInfo{
				Type:    "coach_inactive",
				Message: "教练当前状态不可授课",
			})
		}
	}

	var course models.Course
	if err := db.First(&course, s.CourseID).Error; err != nil {
		conflicts = append(conflicts, ConflictInfo{
			Type:    "course_not_found",
			Message: "课程不存在",
		})
	}

	if len(conflicts) > 0 {
		return ValidateResult{OK: false, Conflicts: conflicts}, conflicts
	}

	q := db.Model(&models.Schedule{}).
		Where("venue_id = ? AND schedule_date = ? AND status <> ?", s.VenueID, s.ScheduleDate, "cancelled")
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var venueConflictCount int64
	q.Where("start_hour < ? AND end_hour > ?", s.EndHour, s.StartHour).Count(&venueConflictCount)
	if venueConflictCount > 0 {
		conflicts = append(conflicts, ConflictInfo{
			Type:    "venue_conflict",
			Message: "该场馆此时段已有课程安排",
		})
	}

	var bookingConflictCount int64
	db.Model(&models.Booking{}).
		Where("venue_id = ? AND book_date = ? AND status <> ?", s.VenueID, s.ScheduleDate, "cancelled").
		Where("start_hour < ? AND end_hour > ?", s.EndHour, s.StartHour).
		Count(&bookingConflictCount)
	if bookingConflictCount > 0 {
		conflicts = append(conflicts, ConflictInfo{
			Type:    "booking_conflict",
			Message: "该场馆此时段已有散客预订",
		})
	}

	q2 := db.Model(&models.Schedule{}).
		Where("coach_id = ? AND schedule_date = ? AND status <> ?", s.CoachID, s.ScheduleDate, "cancelled")
	if excludeID > 0 {
		q2 = q2.Where("id <> ?", excludeID)
	}
	var coachConflictCount int64
	q2.Where("start_hour < ? AND end_hour > ?", s.EndHour, s.StartHour).Count(&coachConflictCount)
	if coachConflictCount > 0 {
		conflicts = append(conflicts, ConflictInfo{
			Type:    "coach_conflict",
			Message: "该教练此时段已有课程安排",
		})
	}

	var leaveConflictCount int64
	db.Model(&models.CoachLeave{}).
		Where("coach_id = ? AND leave_date = ? AND status = ?", s.CoachID, s.ScheduleDate, "approved").
		Where("start_hour < ? AND end_hour > ?", s.EndHour, s.StartHour).
		Count(&leaveConflictCount)
	if leaveConflictCount > 0 {
		conflicts = append(conflicts, ConflictInfo{
			Type:    "coach_leave_conflict",
			Message: "该教练此时段处于请假中",
		})
	}

	durationHours := s.EndHour - s.StartHour
	var dailyHours float64
	dailyQ := db.Model(&models.Schedule{}).
		Where("coach_id = ? AND schedule_date = ? AND status <> ?", s.CoachID, s.ScheduleDate, "cancelled")
	if excludeID > 0 {
		dailyQ = dailyQ.Where("id <> ?", excludeID)
	}
	type durSum struct {
		Total int
	}
	var ds durSum
	dailyQ.Select("COALESCE(SUM(end_hour - start_hour),0) as total").Scan(&ds)
	dailyHours = float64(ds.Total + durationHours)
	if coach.DailyMaxHours > 0 && dailyHours > float64(coach.DailyMaxHours) {
		conflicts = append(conflicts, ConflictInfo{
			Type:    "coach_daily_overload",
			Message: fmt.Sprintf("教练日课时%.1fh将超出上限%.1fh", dailyHours, float64(coach.DailyMaxHours)),
		})
	}

	if date, err := time.Parse("2006-01-02", s.ScheduleDate); err == nil {
		weekday := date.Weekday()
		monday := date.AddDate(0, 0, -int(weekday)+1)
		if weekday == 0 {
			monday = date.AddDate(0, 0, -6)
		}
		sunday := monday.AddDate(0, 0, 6)
		mondayStr := monday.Format("2006-01-02")
		sundayStr := sunday.Format("2006-01-02")

		weeklyQ := db.Model(&models.Schedule{}).
			Where("coach_id = ? AND schedule_date >= ? AND schedule_date <= ? AND status <> ?",
				s.CoachID, mondayStr, sundayStr, "cancelled")
		if excludeID > 0 {
			weeklyQ = weeklyQ.Where("id <> ?", excludeID)
		}
		var ws durSum
		weeklyQ.Select("COALESCE(SUM(end_hour - start_hour),0) as total").Scan(&ws)
		weeklyHours := float64(ws.Total + durationHours)
		if coach.WeeklyMaxHours > 0 && weeklyHours > float64(coach.WeeklyMaxHours) {
			conflicts = append(conflicts, ConflictInfo{
				Type:    "coach_weekly_overload",
				Message: fmt.Sprintf("教练周课时%.1fh将超出上限%.1fh", weeklyHours, float64(coach.WeeklyMaxHours)),
			})
		}
	}

	if len(conflicts) > 0 {
		return ValidateResult{OK: false, Conflicts: conflicts}, conflicts
	}
	return ValidateResult{OK: true}, nil
}

type ScheduleCandidate struct {
	CourseID    uint   `json:"course_id" binding:"required"`
	CoachID     uint   `json:"coach_id"`
	VenueID     uint   `json:"venue_id"`
	PrefDate    string `json:"pref_date"`
	PrefStart   int    `json:"pref_start"`
	PrefEnd     int    `json:"pref_end"`
	MinDate     string `json:"min_date"`
	MaxDate     string `json:"max_date"`
	SportType   string `json:"sport_type"`
	SessionIdx  int    `json:"session_idx"`
}

type ScheduledResult struct {
	Planned    []models.Schedule `json:"planned"`
	Failed     []ScheduleCandidate `json:"failed"`
	FailedReasons map[int]string `json:"failed_reasons"`
	Stats      map[string]interface{} `json:"stats"`
}

type SmartScheduler struct {
	DB *gorm.DB
}

func NewSmartScheduler(db *gorm.DB) *SmartScheduler {
	return &SmartScheduler{DB: db}
}

func (s *SmartScheduler) AutoSchedule(candidates []ScheduleCandidate) ScheduledResult {
	result := ScheduledResult{
		FailedReasons: make(map[int]string),
		Stats:         make(map[string]interface{}),
	}

	type wrapper struct {
		idx  int
		cand ScheduleCandidate
	}
	wrappers := make([]wrapper, len(candidates))
	venueUsage := make(map[uint]int)
	coachSlots := make(map[uint]int)
	for i, c := range candidates {
		wrappers[i] = wrapper{i, c}
		if c.VenueID > 0 {
			venueUsage[c.VenueID]++
		}
		if c.CoachID > 0 {
			coachSlots[c.CoachID]++
		}
	}
	sort.Slice(wrappers, func(i, j int) bool {
		ci := wrappers[i].cand
		cj := wrappers[j].cand
		si := 0
		sj := 0
		if ci.CoachID > 0 {
			si++
		}
		if ci.VenueID > 0 {
			si++
		}
		if ci.PrefStart != 0 && ci.PrefEnd != 0 {
			si++
		}
		if cj.CoachID > 0 {
			sj++
		}
		if cj.VenueID > 0 {
			sj++
		}
		if cj.PrefStart != 0 && cj.PrefEnd != 0 {
			sj++
		}
		return si > sj
	})

	var courses []models.Course
	s.DB.Find(&courses)
	courseMap := make(map[uint]models.Course)
	for _, c := range courses {
		courseMap[c.ID] = c
	}

	var venues []models.Venue
	s.DB.Where("status = ?", "open").Find(&venues)
	var coaches []models.Coach
	s.DB.Where("status = ?", "active").Find(&coaches)

	for _, w := range wrappers {
		cand := w.cand
		course, ok := courseMap[cand.CourseID]
		if !ok {
			result.Failed = append(result.Failed, cand)
			result.FailedReasons[w.idx] = "课程不存在"
			continue
		}

		var targetVenues []models.Venue
		if cand.VenueID > 0 {
			for _, v := range venues {
				if v.ID == cand.VenueID {
					targetVenues = append(targetVenues, v)
					break
				}
			}
		} else {
			st := cand.SportType
			if st == "" {
				st = course.SportType
			}
			for _, v := range venues {
				if st == "" || v.SportType == st {
					targetVenues = append(targetVenues, v)
				}
			}
			sort.Slice(targetVenues, func(i, j int) bool {
				return venueUsage[targetVenues[i].ID] < venueUsage[targetVenues[j].ID]
			})
		}

		var targetCoaches []models.Coach
		if cand.CoachID > 0 {
			for _, c := range coaches {
				if c.ID == cand.CoachID {
					targetCoaches = append(targetCoaches, c)
					break
				}
			}
		} else {
			for _, c := range coaches {
				targetCoaches = append(targetCoaches, c)
			}
			sort.Slice(targetCoaches, func(i, j int) bool {
				return coachSlots[targetCoaches[i].ID] < coachSlots[targetCoaches[j].ID]
			})
		}

		dates := generateDateRange(cand.MinDate, cand.MaxDate, cand.PrefDate)
		duration := course.Duration
		if duration == 0 {
			duration = 1
		}

		found := false
		for _, dt := range dates {
			for _, venue := range targetVenues {
				startOptions := buildStartOptions(cand.PrefStart, cand.PrefEnd,
					venue.OpenHour, venue.CloseHour, duration)
				for _, startHour := range startOptions {
					endHour := startHour + duration
					for _, coach := range targetCoaches {
						sched := models.Schedule{
							CourseID:     course.ID,
							CoachID:      coach.ID,
							VenueID:      venue.ID,
							ScheduleDate: dt,
							StartHour:    startHour,
							EndHour:      endHour,
							Capacity:     course.Capacity,
							Enrolled:     0,
							Status:       "scheduled",
							SessionIndex: cand.SessionIdx,
						}
						vr, _ := CheckConflicts(s.DB, &sched, 0)
						if vr.OK {
							result.Planned = append(result.Planned, sched)
							venueUsage[venue.ID]++
							coachSlots[coach.ID]++
							found = true
							break
						}
					}
					if found {
						break
					}
				}
				if found {
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			result.Failed = append(result.Failed, cand)
			result.FailedReasons[w.idx] = "未找到满足所有约束的时段"
		}
	}

	totalVenueHours := 0
	for _, sched := range result.Planned {
		totalVenueHours += sched.EndHour - sched.StartHour
	}
	result.Stats["scheduled_count"] = len(result.Planned)
	result.Stats["failed_count"] = len(result.Failed)
	result.Stats["total_venue_hours"] = totalVenueHours
	successRate := 0.0
	if len(candidates) > 0 {
		successRate = float64(len(result.Planned)) / float64(len(candidates)) * 100
	}
	result.Stats["success_rate"] = fmt.Sprintf("%.1f%%", successRate)

	return result
}

func generateDateRange(minStr, maxStr, prefStr string) []string {
	var result []string
	if prefStr != "" {
		result = append(result, prefStr)
	}
	if minStr == "" {
		minStr = time.Now().Format("2006-01-02")
	}
	if maxStr == "" {
		min, _ := time.Parse("2006-01-02", minStr)
		maxStr = min.AddDate(0, 0, 14).Format("2006-01-02")
	}
	min, _ := time.Parse("2006-01-02", minStr)
	max, _ := time.Parse("2006-01-02", maxStr)
	for d := min; !d.After(max); d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		if ds != prefStr {
			result = append(result, ds)
		}
	}
	if len(result) == 0 && prefStr != "" {
		result = append(result, prefStr)
	}
	return result
}

func buildStartOptions(prefStart, prefEnd, openHour, closeHour, duration int) []int {
	var result []int
	preferredStart := 0
	preferredEnd := 0
	if prefStart != 0 && prefEnd != 0 {
		preferredStart = prefStart
		preferredEnd = prefEnd - duration
	}
	if preferredStart > 0 && preferredEnd >= preferredStart {
		for h := preferredStart; h <= preferredEnd; h++ {
			if h >= openHour && h+duration <= closeHour {
				result = append(result, h)
			}
		}
	}
	for h := openHour; h+duration <= closeHour; h++ {
		exists := false
		for _, r := range result {
			if r == h {
				exists = true
				break
			}
		}
		if !exists {
			result = append(result, h)
		}
	}
	return result
}
