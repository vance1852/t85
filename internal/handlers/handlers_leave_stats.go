package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"venue-booking-admin/internal/models"
)

type leaveReq struct {
	CoachID   uint   `json:"coach_id" binding:"required"`
	LeaveDate string `json:"leave_date" binding:"required"`
	StartHour int    `json:"start_hour" binding:"required"`
	EndHour   int    `json:"end_hour" binding:"required"`
	Reason    string `json:"reason"`
	Status    string `json:"status"`
}

func (h *Handler) ListLeaves(c *gin.Context) {
	var leaves []models.CoachLeave
	q := h.DB.Order("id DESC")
	if cid := c.Query("coach_id"); cid != "" {
		q = q.Where("coach_id = ?", cid)
	}
	if date := c.Query("date"); date != "" {
		q = q.Where("leave_date = ?", date)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	q.Find(&leaves)
	c.JSON(http.StatusOK, leaves)
}

func (h *Handler) CreateLeave(c *gin.Context) {
	var req leaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	if req.EndHour <= req.StartHour {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "结束时间须晚于开始时间"})
		return
	}
	status := req.Status
	if status == "" {
		status = "pending"
	}
	leave := models.CoachLeave{
		CoachID:   req.CoachID,
		LeaveDate: req.LeaveDate,
		StartHour: req.StartHour,
		EndHour:   req.EndHour,
		Reason:    req.Reason,
		Status:    status,
		Handled:   status != "pending",
	}
	h.DB.Create(&leave)
	c.JSON(http.StatusCreated, leave)
}

func (h *Handler) ApproveLeave(c *gin.Context) {
	var leave models.CoachLeave
	if err := h.DB.First(&leave, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "请假记录不存在"})
		return
	}

	tx := h.DB.Begin()
	leave.Status = "approved"
	leave.Handled = true
	if err := tx.Save(&leave).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "保存失败"})
		return
	}

	var affectedSchedules []models.Schedule
	tx.Where("coach_id = ? AND schedule_date = ? AND status = ?",
		leave.CoachID, leave.LeaveDate, "scheduled").
		Where("start_hour < ? AND end_hour > ?", leave.EndHour, leave.StartHour).
		Find(&affectedSchedules)

	type actionItem struct {
		ScheduleID    uint   `json:"schedule_id"`
		Action        string `json:"action"`
		NewScheduleID *uint  `json:"new_schedule_id,omitempty"`
		Message       string `json:"message"`
	}
	var actions []actionItem

	for _, s := range affectedSchedules {
		var replacementID *uint
		action := "cancelled"
		msg := "教练请假，课程已取消"

		var subs []models.Coach
		tx.Where("status = ? AND id <> ?", "active", leave.CoachID).
			Where("skills LIKE ?", "%"+h.getCourseSport(tx, s.CourseID)+"%").
			Find(&subs)

		var foundSub *uint
		for _, sub := range subs {
			var cnt int64
			tx.Model(&models.Schedule{}).
				Where("coach_id = ? AND schedule_date = ? AND status <> ?",
					sub.ID, s.ScheduleDate, "cancelled").
				Where("start_hour < ? AND end_hour > ?", s.EndHour, s.StartHour).
				Count(&cnt)

			var leaveCnt int64
			tx.Model(&models.CoachLeave{}).
				Where("coach_id = ? AND leave_date = ? AND status = ?",
					sub.ID, s.ScheduleDate, "approved").
				Where("start_hour < ? AND end_hour > ?", s.EndHour, s.StartHour).
				Count(&leaveCnt)

			if cnt == 0 && leaveCnt == 0 {
				schedCopy := s
				schedCopy.ID = 0
				schedCopy.CoachID = sub.ID
				schedCopy.Status = "rescheduled"
				schedCopy.Notes = "请假调课替换教练: " + sub.Name
				if err := tx.Create(&schedCopy).Error; err == nil {
					foundSub = &schedCopy.ID
					break
				}
			}
		}

		if foundSub != nil {
			replacementID = foundSub
			action = "rescheduled"
			msg = "已自动替换教练并调课"
			var enrollments []models.Enrollment
			tx.Where("schedule_id = ? AND status = ?", s.ID, "enrolled").
				Find(&enrollments)
			for _, e := range enrollments {
				e2 := e
				e2.ID = 0
				e2.ScheduleID = *foundSub
				e2.Status = "transferred"
				tx.Create(&e2)
			}
			tx.Model(&models.Enrollment{}).
				Where("schedule_id = ? AND status = ?", s.ID, "enrolled").
				Updates(map[string]interface{}{"status": "transferred"})
		}

		oldStatus := s.Status
		s.Status = "cancelled"
		tx.Save(&s)

		if action == "cancelled" {
			var enrolls []models.Enrollment
			tx.Where("schedule_id = ? AND status = ?", s.ID, "enrolled").Find(&enrolls)
			for _, e := range enrolls {
				e.Status = "refund_pending"
				e.RefundAmount = e.PricePaid
				tx.Save(&e)
			}
			var waits []models.Enrollment
			tx.Where("schedule_id = ? AND status = ?", s.ID, "waitlisted").Find(&waits)
			for _, e := range waits {
				e.Status = "cancelled"
				tx.Save(&e)
			}
			_ = oldStatus
		}

		tx.Create(&models.ScheduleAdjustment{
			OriginalScheduleID: s.ID,
			NewScheduleID:      replacementID,
			AdjustType:         "leave_" + action,
			Reason:             "教练请假: " + leave.Reason,
			HandledBy:          "system",
		})

		actions = append(actions, actionItem{
			ScheduleID:    s.ID,
			Action:        action,
			NewScheduleID: replacementID,
			Message:       msg,
		})
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"leave":             leave,
		"affected_count":    len(affectedSchedules),
		"schedule_actions":  actions,
	})
}

func (h *Handler) getCourseSport(db *gorm.DB, courseID uint) string {
	var course models.Course
	if err := db.First(&course, courseID).Error; err == nil {
		return course.SportType
	}
	return ""
}

func (h *Handler) RejectLeave(c *gin.Context) {
	var leave models.CoachLeave
	if err := h.DB.First(&leave, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "请假记录不存在"})
		return
	}
	leave.Status = "rejected"
	leave.Handled = true
	h.DB.Save(&leave)
	c.JSON(http.StatusOK, leave)
}

func (h *Handler) RescheduleClass(c *gin.Context) {
	var req struct {
		ScheduleID       uint   `json:"schedule_id" binding:"required"`
		NewCoachID       uint   `json:"new_coach_id"`
		NewVenueID       uint   `json:"new_venue_id"`
		NewDate          string `json:"new_date"`
		NewStartHour     int    `json:"new_start_hour"`
		NewEndHour       int    `json:"new_end_hour"`
		Reason           string `json:"reason"`
		TransferEnrolls  bool   `json:"transfer_enrolls"`
		RefundRemaining  bool   `json:"refund_remaining"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}

	var original models.Schedule
	if err := h.DB.First(&original, req.ScheduleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "原排课不存在"})
		return
	}

	newSched := original
	newSched.ID = 0
	if req.NewCoachID > 0 {
		newSched.CoachID = req.NewCoachID
	}
	if req.NewVenueID > 0 {
		newSched.VenueID = req.NewVenueID
	}
	if req.NewDate != "" {
		newSched.ScheduleDate = req.NewDate
	}
	if req.NewStartHour > 0 {
		newSched.StartHour = req.NewStartHour
	}
	if req.NewEndHour > 0 {
		newSched.EndHour = req.NewEndHour
	}
	newSched.Enrolled = 0
	newSched.Status = "rescheduled"
	newSched.Notes = req.Reason

	tx := h.DB.Begin()
	if err := tx.Create(&newSched).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "创建新排课失败: " + err.Error()})
		return
	}

	transferred := 0
	if req.TransferEnrolls {
		var enrolls []models.Enrollment
		tx.Where("schedule_id = ? AND status = ?", original.ID, "enrolled").Find(&enrolls)
		for _, e := range enrolls {
			if newSched.Enrolled >= newSched.Capacity {
				if req.RefundRemaining {
					e.Status = "refund_pending"
					e.RefundAmount = e.PricePaid
					tx.Save(&e)
				}
				continue
			}
			e2 := e
			e2.ID = 0
			e2.ScheduleID = newSched.ID
			e2.Status = "transferred"
			e2.CheckedIn = false
			e2.CheckedInAt = nil
			tx.Create(&e2)
			newSched.Enrolled++
			transferred++
		}
		tx.Save(&newSched)

		if req.RefundRemaining {
			var remainingEnrolls []models.Enrollment
			tx.Where("schedule_id = ? AND status = ?", original.ID, "enrolled").
				Find(&remainingEnrolls)
			for _, e := range remainingEnrolls {
				e.Status = "refund_pending"
				e.RefundAmount = e.PricePaid
				tx.Save(&e)
			}
		} else {
			tx.Model(&models.Enrollment{}).
				Where("schedule_id = ? AND status = ?", original.ID, "enrolled").
				Update("status", "transferred")
		}
	}

	original.Status = "cancelled"
	tx.Save(&original)

	tx.Create(&models.ScheduleAdjustment{
		OriginalScheduleID: original.ID,
		NewScheduleID:      &newSched.ID,
		AdjustType:         "reschedule",
		Reason:             req.Reason,
		HandledBy:          "admin",
	})

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"original_schedule": original,
		"new_schedule":      newSched,
		"transferred_count": transferred,
	})
}

type refundReq struct {
	Amount float64 `json:"amount"`
	Reason string  `json:"reason"`
}

func (h *Handler) ProcessRefund(c *gin.Context) {
	var enroll models.Enrollment
	if err := h.DB.First(&enroll, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "报名记录不存在"})
		return
	}
	if enroll.Status != "refund_pending" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "该报名不处于待退费状态"})
		return
	}
	var req refundReq
	c.ShouldBindJSON(&req)
	amount := req.Amount
	if amount <= 0 {
		amount = enroll.RefundAmount
	}
	if amount <= 0 {
		amount = enroll.PricePaid
	}
	enroll.Status = "refunded"
	enroll.RefundAmount = amount
	h.DB.Save(&enroll)
	c.JSON(http.StatusOK, gin.H{"enrollment": enroll})
}

func (h *Handler) ListAdjustments(c *gin.Context) {
	var list []models.ScheduleAdjustment
	q := h.DB.Order("id DESC")
	if sid := c.Query("schedule_id"); sid != "" {
		q = q.Where("original_schedule_id = ? OR new_schedule_id = ?", sid, sid)
	}
	q.Find(&list)
	c.JSON(http.StatusOK, list)
}

func (h *Handler) AttendanceRate(c *gin.Context) {
	var result []struct {
		ScheduleID   uint    `json:"schedule_id"`
		CourseName   string  `json:"course_name"`
		CoachName    string  `json:"coach_name"`
		VenueName    string  `json:"venue_name"`
		ScheduleDate string  `json:"schedule_date"`
		StartHour    int     `json:"start_hour"`
		Capacity     int     `json:"capacity"`
		Enrolled     int     `json:"enrolled"`
		CheckedIn    int     `json:"checked_in"`
		Rate         float64 `json:"rate"`
	}
	q := h.DB.Table("schedules s").
		Select(`s.id as schedule_id, c.name as course_name, co.name as coach_name,
			    v.name as venue_name, s.schedule_date, s.start_hour, s.capacity, s.enrolled,
			    COUNT(CASE WHEN e.checked_in = true THEN 1 END) as checked_in,
			    CASE WHEN s.enrolled > 0 
			         THEN ROUND(COUNT(CASE WHEN e.checked_in = true THEN 1 END)*100.0/s.enrolled, 2)
			         ELSE 0 END as rate`).
		Joins("LEFT JOIN courses c ON s.course_id = c.id").
		Joins("LEFT JOIN coaches co ON s.coach_id = co.id").
		Joins("LEFT JOIN venues v ON s.venue_id = v.id").
		Joins("LEFT JOIN enrollments e ON e.schedule_id = s.id AND e.status IN ('enrolled','transferred')").
		Group("s.id")
	if df := c.Query("date_from"); df != "" {
		q = q.Where("s.schedule_date >= ?", df)
	}
	if dt := c.Query("date_to"); dt != "" {
		q = q.Where("s.schedule_date <= ?", dt)
	}
	if cid := c.Query("coach_id"); cid != "" {
		q = q.Where("s.coach_id = ?", cid)
	}
	q.Order("s.schedule_date DESC, s.start_hour ASC").Scan(&result)

	var totalEnrolled, totalChecked int
	for _, r := range result {
		totalEnrolled += r.Enrolled
		totalChecked += r.CheckedIn
	}
	overall := 0.0
	if totalEnrolled > 0 {
		overall = float64(totalChecked) * 100.0 / float64(totalEnrolled)
	}

	c.JSON(http.StatusOK, gin.H{
		"detail":              result,
		"total_enrolled":      totalEnrolled,
		"total_checked_in":    totalChecked,
		"overall_attendance":  overall,
	})
}

func (h *Handler) CoachWorkload(c *gin.Context) {
	var result []struct {
		CoachID         uint    `json:"coach_id"`
		CoachName       string  `json:"coach_name"`
		ScheduleCount   int     `json:"schedule_count"`
		TeachingHours   float64 `json:"teaching_hours"`
		TotalStudents   int     `json:"total_students"`
		AttendedStudents int    `json:"attended_students"`
		AttendanceRate  float64 `json:"attendance_rate"`
		EstimatedIncome float64 `json:"estimated_income"`
	}
	q := h.DB.Table("schedules s").
		Select(`s.coach_id, co.name as coach_name,
			    COUNT(DISTINCT s.id) as schedule_count,
			    COALESCE(SUM(s.end_hour - s.start_hour), 0) as teaching_hours,
			    COALESCE(SUM(s.enrolled), 0) as total_students,
			    COUNT(CASE WHEN e.checked_in = true THEN 1 END) as attended_students,
			    CASE WHEN COALESCE(SUM(s.enrolled),0) > 0
			         THEN ROUND(COUNT(CASE WHEN e.checked_in = true THEN 1 END)*100.0/SUM(s.enrolled), 2)
			         ELSE 0 END as attendance_rate,
			    COALESCE(SUM(e.price_paid), 0) as estimated_income`).
		Joins("LEFT JOIN coaches co ON s.coach_id = co.id").
		Joins("LEFT JOIN enrollments e ON e.schedule_id = s.id AND e.status IN ('enrolled','transferred')").
		Where("s.status <> ?", "cancelled").
		Group("s.coach_id")
	if df := c.Query("date_from"); df != "" {
		q = q.Where("s.schedule_date >= ?", df)
	}
	if dt := c.Query("date_to"); dt != "" {
		q = q.Where("s.schedule_date <= ?", dt)
	}
	if cid := c.Query("coach_id"); cid != "" {
		q = q.Where("s.coach_id = ?", cid)
	}
	q.Order("teaching_hours DESC").Scan(&result)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) RevenueStats(c *gin.Context) {
	var result struct {
		TotalRevenue        float64 `json:"total_revenue"`
		BookingRevenue      float64 `json:"booking_revenue"`
		CourseRevenue       float64 `json:"course_revenue"`
		RefundAmount        float64 `json:"refund_amount"`
		NetRevenue          float64 `json:"net_revenue"`
		PaidCount           int64   `json:"paid_count"`
		RefundCount         int64   `json:"refund_count"`
		ByCourseType        []struct {
			CourseType   string  `json:"course_type"`
			Revenue      float64 `json:"revenue"`
			EnrollCount  int64   `json:"enroll_count"`
		} `json:"by_course_type"`
		BySportType         []struct {
			SportType   string  `json:"sport_type"`
			Revenue     float64 `json:"revenue"`
		} `json:"by_sport_type"`
		ByDate              []struct {
			Date    string  `json:"date"`
			Revenue float64 `json:"revenue"`
		} `json:"by_date"`
	}

	h.DB.Model(&models.Booking{}).
		Where("status <> ?", "cancelled").
		Select("COALESCE(SUM(amount),0)").Scan(&result.BookingRevenue)

	enrollQ := h.DB.Model(&models.Enrollment{}).
		Where("status IN ?", []string{"enrolled", "transferred", "completed", "refunded"})
	if df := c.Query("date_from"); df != "" {
		enrollQ = enrollQ.Where("DATE(created_at) >= ?", df)
	}
	if dt := c.Query("date_to"); dt != "" {
		enrollQ = enrollQ.Where("DATE(created_at) <= ?", dt)
	}
	enrollQ.Select("COALESCE(SUM(price_paid),0)").Scan(&result.CourseRevenue)

	h.DB.Model(&models.Enrollment{}).
		Where("status = ?", "refunded").
		Select("COALESCE(SUM(refund_amount),0)").Scan(&result.RefundAmount)

	h.DB.Model(&models.Enrollment{}).
		Where("status IN ?", []string{"enrolled", "transferred", "completed"}).
		Count(&result.PaidCount)

	h.DB.Model(&models.Enrollment{}).
		Where("status = ?", "refunded").
		Count(&result.RefundCount)

	result.TotalRevenue = result.BookingRevenue + result.CourseRevenue
	result.NetRevenue = result.TotalRevenue - result.RefundAmount

	h.DB.Table("enrollments e").
		Select(`c.course_type, COALESCE(SUM(e.price_paid),0) as revenue, COUNT(*) as enroll_count`).
		Joins("LEFT JOIN courses c ON e.course_id = c.id").
		Where("e.status IN ?", []string{"enrolled", "transferred", "completed", "refunded"}).
		Group("c.course_type").
		Scan(&result.ByCourseType)

	h.DB.Table("enrollments e").
		Select(`c.sport_type, COALESCE(SUM(e.price_paid),0) as revenue`).
		Joins("LEFT JOIN courses c ON e.course_id = c.id").
		Where("e.status IN ?", []string{"enrolled", "transferred", "completed", "refunded"}).
		Group("c.sport_type").
		Scan(&result.BySportType)

	h.DB.Table(`(
		SELECT DATE(created_at) as date, price_paid as amount FROM enrollments
		WHERE status IN ('enrolled','transferred','completed','refunded')
		UNION ALL
		SELECT DATE(created_at) as date, amount FROM bookings WHERE status <> 'cancelled'
	) combined`).
		Select("date, COALESCE(SUM(amount),0) as revenue").
		Group("date").Order("date DESC").Limit(30).
		Scan(&result.ByDate)

	c.JSON(http.StatusOK, result)
}

func (h *Handler) EnrollmentDashboard(c *gin.Context) {
	var result struct {
		TotalStudents       int64   `json:"total_students"`
		TotalCourses        int64   `json:"total_courses"`
		ActiveSchedules     int64   `json:"active_schedules"`
		TotalEnrollments    int64   `json:"total_enrollments"`
		WaitlistCount       int64   `json:"waitlist_count"`
		TodaySchedules      int64   `json:"today_schedules"`
		TodayEnrollments    int64   `json:"today_enrollments"`
		RefundPending       int64   `json:"refund_pending"`
	}
	h.DB.Model(&models.Student{}).Count(&result.TotalStudents)
	h.DB.Model(&models.Course{}).Where("status = ?", "active").Count(&result.TotalCourses)
	h.DB.Model(&models.Schedule{}).Where("status IN ?", []string{"scheduled", "in_progress"}).Count(&result.ActiveSchedules)
	h.DB.Model(&models.Enrollment{}).Where("status IN ?", []string{"enrolled", "transferred"}).Count(&result.TotalEnrollments)
	h.DB.Model(&models.Enrollment{}).Where("status = ?", "waitlisted").Count(&result.WaitlistCount)
	today := time.Now().Format("2006-01-02")
	h.DB.Model(&models.Schedule{}).Where("schedule_date = ?", today).Count(&result.TodaySchedules)
	h.DB.Model(&models.Enrollment{}).Where("DATE(created_at) = ?", today).Count(&result.TodayEnrollments)
	h.DB.Model(&models.Enrollment{}).Where("status = ?", "refund_pending").Count(&result.RefundPending)
	c.JSON(http.StatusOK, result)
}
