package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"venue-booking-admin/internal/models"
	"venue-booking-admin/internal/scheduler"
)

type scheduleReq struct {
	CourseID     uint   `json:"course_id" binding:"required"`
	CoachID      uint   `json:"coach_id" binding:"required"`
	VenueID      uint   `json:"venue_id" binding:"required"`
	ScheduleDate string `json:"schedule_date" binding:"required"`
	StartHour    int    `json:"start_hour" binding:"required"`
	EndHour      int    `json:"end_hour" binding:"required"`
	Capacity     int    `json:"capacity"`
	SessionIndex int    `json:"session_index"`
	Notes        string `json:"notes"`
}

type scheduleValidateReq struct {
	ID           uint   `json:"id"`
	CourseID     uint   `json:"course_id" binding:"required"`
	CoachID      uint   `json:"coach_id" binding:"required"`
	VenueID      uint   `json:"venue_id" binding:"required"`
	ScheduleDate string `json:"schedule_date" binding:"required"`
	StartHour    int    `json:"start_hour" binding:"required"`
	EndHour      int    `json:"end_hour" binding:"required"`
}

type smartScheduleReq struct {
	Candidates []scheduler.ScheduleCandidate `json:"candidates" binding:"required"`
	Persist    bool                         `json:"persist"`
}

type scheduleStatusReq struct {
	Status string `json:"status" binding:"required"`
}

func (h *Handler) ListSchedules(c *gin.Context) {
	var schedules []models.Schedule
	q := h.DB.Preload("Coach").Preload("Venue").Preload("Course").Order("schedule_date DESC, start_hour ASC")
	if cid := c.Query("course_id"); cid != "" {
		q = q.Where("course_id = ?", cid)
	}
	if cid2 := c.Query("coach_id"); cid2 != "" {
		q = q.Where("coach_id = ?", cid2)
	}
	if vid := c.Query("venue_id"); vid != "" {
		q = q.Where("venue_id = ?", vid)
	}
	if date := c.Query("date"); date != "" {
		q = q.Where("schedule_date = ?", date)
	}
	if dateFrom := c.Query("date_from"); dateFrom != "" {
		q = q.Where("schedule_date >= ?", dateFrom)
	}
	if dateTo := c.Query("date_to"); dateTo != "" {
		q = q.Where("schedule_date <= ?", dateTo)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	q.Find(&schedules)
	c.JSON(http.StatusOK, schedules)
}

func (h *Handler) ValidateSchedule(c *gin.Context) {
	var req scheduleValidateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	s := models.Schedule{
		ID:           req.ID,
		CourseID:     req.CourseID,
		CoachID:      req.CoachID,
		VenueID:      req.VenueID,
		ScheduleDate: req.ScheduleDate,
		StartHour:    req.StartHour,
		EndHour:      req.EndHour,
	}
	result, _ := scheduler.CheckConflicts(h.DB, &s, req.ID)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) CreateSchedule(c *gin.Context) {
	var req scheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}

	var course models.Course
	if err := h.DB.First(&course, req.CourseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "课程不存在"})
		return
	}

	capacity := req.Capacity
	if capacity <= 0 {
		capacity = course.Capacity
	}

	s := models.Schedule{
		CourseID:     req.CourseID,
		CoachID:      req.CoachID,
		VenueID:      req.VenueID,
		ScheduleDate: req.ScheduleDate,
		StartHour:    req.StartHour,
		EndHour:      req.EndHour,
		Capacity:     capacity,
		Enrolled:     0,
		Status:       "scheduled",
		SessionIndex: req.SessionIndex,
		Notes:        req.Notes,
	}

	vr, conflicts := scheduler.CheckConflicts(h.DB, &s, 0)
	if !vr.OK {
		c.JSON(http.StatusConflict, gin.H{
			"detail":    "存在排课冲突",
			"conflicts": conflicts,
		})
		return
	}

	h.DB.Create(&s)
	h.DB.Preload("Coach").Preload("Venue").Preload("Course").First(&s, s.ID)
	c.JSON(http.StatusCreated, s)
}

func (h *Handler) GetSchedule(c *gin.Context) {
	var s models.Schedule
	if err := h.DB.Preload("Coach").Preload("Venue").Preload("Course").
		First(&s, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "排课不存在"})
		return
	}
	type enrollmentInfo struct {
		models.Enrollment
		StudentName string `json:"student_name"`
		StudentPhone string `json:"student_phone"`
	}
	var enrollments []enrollmentInfo
	h.DB.Table("enrollments e").
		Select("e.*, s.name as student_name, s.phone as student_phone").
		Joins("LEFT JOIN students s ON e.student_id = s.id").
		Where("e.schedule_id = ?", s.ID).
		Order("e.status, e.waitlist_pos, e.id").
		Scan(&enrollments)
	c.JSON(http.StatusOK, gin.H{
		"schedule":    s,
		"enrollments": enrollments,
	})
}

func (h *Handler) UpdateSchedule(c *gin.Context) {
	var s models.Schedule
	if err := h.DB.First(&s, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "排课不存在"})
		return
	}
	var req scheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	if s.Enrolled > req.Capacity && req.Capacity > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "容量不能小于已报名人数"})
		return
	}

	updated := models.Schedule{
		ID:           s.ID,
		CourseID:     req.CourseID,
		CoachID:      req.CoachID,
		VenueID:      req.VenueID,
		ScheduleDate: req.ScheduleDate,
		StartHour:    req.StartHour,
		EndHour:      req.EndHour,
	}

	vr, conflicts := scheduler.CheckConflicts(h.DB, &updated, s.ID)
	if !vr.OK {
		c.JSON(http.StatusConflict, gin.H{
			"detail":    "修改后存在排课冲突",
			"conflicts": conflicts,
		})
		return
	}

	s.CourseID = req.CourseID
	s.CoachID = req.CoachID
	s.VenueID = req.VenueID
	s.ScheduleDate = req.ScheduleDate
	s.StartHour = req.StartHour
	s.EndHour = req.EndHour
	if req.Capacity > 0 {
		s.Capacity = req.Capacity
	}
	if req.SessionIndex != 0 {
		s.SessionIndex = req.SessionIndex
	}
	s.Notes = req.Notes
	h.DB.Save(&s)
	h.DB.Preload("Coach").Preload("Venue").Preload("Course").First(&s, s.ID)
	c.JSON(http.StatusOK, s)
}

func (h *Handler) UpdateScheduleStatus(c *gin.Context) {
	var s models.Schedule
	if err := h.DB.First(&s, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "排课不存在"})
		return
	}
	var req scheduleStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	validStatuses := map[string]bool{
		"scheduled": true, "in_progress": true, "completed": true, "cancelled": true,
	}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "状态不合法"})
		return
	}

	tx := h.DB.Begin()
	oldStatus := s.Status
	s.Status = req.Status

	if req.Status == "cancelled" && oldStatus != "cancelled" {
		var enrollments []models.Enrollment
		tx.Where("schedule_id = ? AND status IN ?", s.ID,
			[]string{"enrolled", "transferred", "from_waitlist"}).Find(&enrollments)
		for _, e := range enrollments {
			e.Status = "refund_pending"
			e.RefundAmount = e.PricePaid
			tx.Save(&e)
		}
		var waitlisted []models.Enrollment
		tx.Where("schedule_id = ? AND status = ?", s.ID, "waitlisted").Find(&waitlisted)
		for _, e := range waitlisted {
			e.Status = "cancelled"
			tx.Save(&e)
		}
		tx.Create(&models.ScheduleAdjustment{
			OriginalScheduleID: s.ID,
			AdjustType:         "cancel",
			Reason:             "管理员取消排课",
			HandledBy:          "admin",
		})
	}

	if err := tx.Save(&s).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "保存失败"})
		return
	}
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"id": s.ID, "status": s.Status})
}

func (h *Handler) DeleteSchedule(c *gin.Context) {
	var s models.Schedule
	if err := h.DB.First(&s, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "排课不存在"})
		return
	}
	if s.Enrolled > 0 && s.Status != "cancelled" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "该排课有学员报名，请先处理报名或取消"})
		return
	}
	h.DB.Delete(&s)
	c.Status(http.StatusNoContent)
}

func (h *Handler) SmartSchedule(c *gin.Context) {
	var req smartScheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	sched := scheduler.NewSmartScheduler(h.DB)
	result := sched.AutoSchedule(req.Candidates)

	if req.Persist && len(result.Planned) > 0 {
		tx := h.DB.Begin()
		for i := range result.Planned {
			if err := tx.Create(&result.Planned[i]).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"detail": "批量排课保存失败: " + err.Error()})
				return
			}
		}
		tx.Commit()
	}

	c.JSON(http.StatusOK, result)
}

func (h *Handler) BatchCreateSchedules(c *gin.Context) {
	var req struct {
		CourseID     uint   `json:"course_id" binding:"required"`
		CoachID      uint   `json:"coach_id" binding:"required"`
		VenueID      uint   `json:"venue_id" binding:"required"`
		StartDate    string `json:"start_date" binding:"required"`
		EndDate      string `json:"end_date" binding:"required"`
		StartHour    int    `json:"start_hour" binding:"required"`
		EndHour      int    `json:"end_hour" binding:"required"`
		Weekdays     []int  `json:"weekdays"`
		Capacity     int    `json:"capacity"`
		SkipConflict bool   `json:"skip_conflict"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	sd, err1 := time.Parse("2006-01-02", req.StartDate)
	ed, err2 := time.Parse("2006-01-02", req.EndDate)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "日期格式不合法"})
		return
	}
	if ed.Before(sd) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "结束日期不能早于开始日期"})
		return
	}
	if len(req.Weekdays) == 0 {
		req.Weekdays = []int{1, 2, 3, 4, 5, 6, 7}
	}
	wdayMap := make(map[int]bool)
	for _, w := range req.Weekdays {
		if w < 1 || w > 7 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "星期范围为1-7"})
			return
		}
		wdayMap[w] = true
	}

	var course models.Course
	if err := h.DB.First(&course, req.CourseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "课程不存在"})
		return
	}
	capacity := req.Capacity
	if capacity <= 0 {
		capacity = course.Capacity
	}

	var created []models.Schedule
	var skipped []map[string]interface{}
	idx := 1

	tx := h.DB.Begin()
	for d := sd; !d.After(ed); d = d.AddDate(0, 0, 1) {
		w := int(d.Weekday())
		if w == 0 {
			w = 7
		}
		if !wdayMap[w] {
			continue
		}
		s := models.Schedule{
			CourseID:     req.CourseID,
			CoachID:      req.CoachID,
			VenueID:      req.VenueID,
			ScheduleDate: d.Format("2006-01-02"),
			StartHour:    req.StartHour,
			EndHour:      req.EndHour,
			Capacity:     capacity,
			Enrolled:     0,
			Status:       "scheduled",
			SessionIndex: idx,
		}
		vr, conflicts := scheduler.CheckConflicts(h.DB, &s, 0)
		if !vr.OK {
			if req.SkipConflict {
				skipped = append(skipped, map[string]interface{}{
					"date":      s.ScheduleDate,
					"conflicts": conflicts,
				})
				continue
			} else {
				tx.Rollback()
				c.JSON(http.StatusConflict, gin.H{
					"detail":    "存在排课冲突",
					"date":      s.ScheduleDate,
					"conflicts": conflicts,
				})
				return
			}
		}
		if err := tx.Create(&s).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "创建排课失败"})
			return
		}
		created = append(created, s)
		idx++
	}
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"created_count": len(created),
		"skipped_count": len(skipped),
		"created":       created,
		"skipped":       skipped,
	})
}

func (h *Handler) SchedulesByCoach(c *gin.Context) {
	coachID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "教练ID不合法"})
		return
	}
	var schedules []models.Schedule
	q := h.DB.Preload("Venue").Preload("Course").
		Where("coach_id = ?", uint(coachID)).
		Order("schedule_date DESC, start_hour ASC")
	if dateFrom := c.Query("date_from"); dateFrom != "" {
		q = q.Where("schedule_date >= ?", dateFrom)
	}
	if dateTo := c.Query("date_to"); dateTo != "" {
		q = q.Where("schedule_date <= ?", dateTo)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	q.Find(&schedules)
	c.JSON(http.StatusOK, schedules)
}

func (h *Handler) SchedulesByVenue(c *gin.Context) {
	venueID, err := strconv.ParseUint(c.Param("venue_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "场馆ID不合法"})
		return
	}
	var schedules []models.Schedule
	q := h.DB.Preload("Coach").Preload("Course").
		Where("venue_id = ?", uint(venueID)).
		Order("schedule_date DESC, start_hour ASC")
	if dateFrom := c.Query("date_from"); dateFrom != "" {
		q = q.Where("schedule_date >= ?", dateFrom)
	}
	if dateTo := c.Query("date_to"); dateTo != "" {
		q = q.Where("schedule_date <= ?", dateTo)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	q.Find(&schedules)
	c.JSON(http.StatusOK, schedules)
}
