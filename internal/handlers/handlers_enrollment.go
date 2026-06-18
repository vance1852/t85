package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"venue-booking-admin/internal/models"
)

type studentReq struct {
	Name   string `json:"name" binding:"required"`
	Phone  string `json:"phone" binding:"required"`
	Email  string `json:"email"`
	Gender string `json:"gender"`
	Age    int    `json:"age"`
	Level  string `json:"level"`
	Notes  string `json:"notes"`
}

func (h *Handler) ListStudents(c *gin.Context) {
	var students []models.Student
	q := h.DB.Order("id DESC")
	if name := c.Query("name"); name != "" {
		q = q.Where("name LIKE ?", "%"+name+"%")
	}
	if phone := c.Query("phone"); phone != "" {
		q = q.Where("phone LIKE ?", "%"+phone+"%")
	}
	if level := c.Query("level"); level != "" {
		q = q.Where("level = ?", level)
	}
	q.Find(&students)
	c.JSON(http.StatusOK, students)
}

func (h *Handler) CreateStudent(c *gin.Context) {
	var req studentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	student := models.Student{
		Name:   req.Name,
		Phone:  req.Phone,
		Email:  req.Email,
		Gender: req.Gender,
		Age:    req.Age,
		Level:  req.Level,
		Notes:  req.Notes,
	}
	if err := h.DB.Create(&student).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"detail": "手机号已存在"})
		return
	}
	c.JSON(http.StatusCreated, student)
}

func (h *Handler) GetStudent(c *gin.Context) {
	var student models.Student
	if err := h.DB.First(&student, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "学员不存在"})
		return
	}
	type enrollRow struct {
		ID            uint      `json:"id"`
		ScheduleID    uint      `json:"schedule_id"`
		CourseID      uint      `json:"course_id"`
		EnrollType    string    `json:"enroll_type"`
		Status        string    `json:"status"`
		WaitlistPos   int       `json:"waitlist_pos"`
		PricePaid     float64   `json:"price_paid"`
		RefundAmount  float64   `json:"refund_amount"`
		CheckedIn     bool      `json:"checked_in"`
		CheckedInAt   *time.Time `json:"checked_in_at"`
		CreatedAt     time.Time `json:"created_at"`
		CourseName    string    `json:"course_name"`
		CourseType    string    `json:"course_type"`
		ScheduleDate  string    `json:"schedule_date"`
		StartHour     int       `json:"start_hour"`
		EndHour       int       `json:"end_hour"`
		ScheduleStatus string   `json:"schedule_status"`
		CoachName     string    `json:"coach_name"`
		VenueName     string    `json:"venue_name"`
	}
	var enrolls []enrollRow
	h.DB.Table("enrollments e").
		Select(`e.id, e.schedule_id, e.course_id, e.enroll_type, e.status, e.waitlist_pos,
			    e.price_paid, e.refund_amount, e.checked_in, e.checked_in_at, e.created_at,
			    c.name as course_name, c.course_type,
			    s.schedule_date, s.start_hour, s.end_hour, s.status as schedule_status,
			    co.name as coach_name, v.name as venue_name`).
		Joins("LEFT JOIN courses c ON e.course_id = c.id").
		Joins("LEFT JOIN schedules s ON e.schedule_id = s.id").
		Joins("LEFT JOIN coaches co ON s.coach_id = co.id").
		Joins("LEFT JOIN venues v ON s.venue_id = v.id").
		Where("e.student_id = ?", student.ID).
		Order("e.id DESC").
		Scan(&enrolls)
	c.JSON(http.StatusOK, gin.H{"student": student, "enrollments": enrolls})
}

func (h *Handler) UpdateStudent(c *gin.Context) {
	var student models.Student
	if err := h.DB.First(&student, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "学员不存在"})
		return
	}
	var req studentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	student.Name = req.Name
	if req.Phone != "" {
		student.Phone = req.Phone
	}
	student.Email = req.Email
	student.Gender = req.Gender
	student.Age = req.Age
	student.Level = req.Level
	student.Notes = req.Notes
	h.DB.Save(&student)
	c.JSON(http.StatusOK, student)
}

func (h *Handler) DeleteStudent(c *gin.Context) {
	var student models.Student
	if err := h.DB.First(&student, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "学员不存在"})
		return
	}
	var count int64
	h.DB.Model(&models.Enrollment{}).
		Where("student_id = ? AND status IN ?", student.ID, []string{"enrolled", "waitlisted"}).
		Count(&count)
	if count > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "该学员还有未结束的课程报名，请先处理"})
		return
	}
	h.DB.Delete(&student)
	c.Status(http.StatusNoContent)
}

type enrollReq struct {
	StudentID  uint    `json:"student_id" binding:"required"`
	ScheduleID uint    `json:"schedule_id" binding:"required"`
	PricePaid  float64 `json:"price_paid"`
}

func (h *Handler) EnrollStudent(c *gin.Context) {
	var req enrollReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}

	unlock := h.lockSchedule(req.ScheduleID)
	defer unlock()

	tx := h.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var schedule models.Schedule
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&schedule, req.ScheduleID).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"detail": "排课不存在"})
		return
	}
	if schedule.Status != "scheduled" && schedule.Status != "in_progress" {
		tx.Rollback()
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "该排课状态不接受报名"})
		return
	}

	var existingCount int64
	tx.Model(&models.Enrollment{}).
		Set("gorm:query_option", "LOCK IN SHARE MODE").
		Where("student_id = ? AND schedule_id = ? AND status IN ?",
			req.StudentID, req.ScheduleID,
			[]string{"enrolled", "waitlisted", "transferred", "from_waitlist"}).
		Count(&existingCount)
	if existingCount > 0 {
		tx.Rollback()
		c.JSON(http.StatusConflict, gin.H{"detail": "该学员已报名或在候补名单中"})
		return
	}

	var course models.Course
	if err := tx.First(&course, schedule.CourseID).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"detail": "关联课程不存在"})
		return
	}

	price := req.PricePaid
	if price <= 0 {
		price = course.Price
	}

	enroll := models.Enrollment{
		StudentID:  req.StudentID,
		ScheduleID: req.ScheduleID,
		CourseID:   schedule.CourseID,
		Status:     "enrolled",
		PricePaid:  price,
	}

	if schedule.Enrolled >= schedule.Capacity {
		var maxPos int
		tx.Model(&models.Enrollment{}).
			Set("gorm:query_option", "FOR UPDATE").
			Where("schedule_id = ? AND status = ?", req.ScheduleID, "waitlisted").
			Select("COALESCE(MAX(waitlist_pos),0)").Scan(&maxPos)
		enroll.Status = "waitlisted"
		enroll.WaitlistPos = maxPos + 1
		enroll.EnrollType = "waitlist"
	} else {
		schedule.Enrolled++
		if err := tx.Save(&schedule).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "更新排课容量失败: " + err.Error()})
			return
		}
		enroll.EnrollType = course.CourseType
	}

	if err := tx.Create(&enroll).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "写入报名失败: " + err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "提交事务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"enrollment": enroll,
		"enrolled":   enroll.Status == "enrolled",
		"waitlisted": enroll.Status == "waitlisted",
	})
}

type cancelEnrollReq struct {
	RefundAmount float64 `json:"refund_amount"`
	Reason       string  `json:"reason"`
}

func (h *Handler) CancelEnrollment(c *gin.Context) {
	var enroll models.Enrollment
	if err := h.DB.First(&enroll, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "报名记录不存在"})
		return
	}
	if enroll.Status == "cancelled" || enroll.Status == "refunded" || enroll.Status == "refund_pending" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "该报名已取消或已退费"})
		return
	}

	var req cancelEnrollReq
	c.ShouldBindJSON(&req)

	tx := h.DB.Begin()

	if enroll.Status == "enrolled" {
		var schedule models.Schedule
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&schedule, enroll.ScheduleID).Error; err == nil {
			if schedule.Enrolled > 0 {
				schedule.Enrolled--
				tx.Save(&schedule)
			}

			var nextWait models.Enrollment
			err := tx.Where("schedule_id = ? AND status = ?", schedule.ID, "waitlisted").
				Order("waitlist_pos ASC").
				First(&nextWait).Error
			if err == nil {
				nextWait.Status = "enrolled"
				nextWait.WaitlistPos = 0
				nextWait.EnrollType = "from_waitlist"
				tx.Save(&nextWait)
				schedule.Enrolled++
				tx.Save(&schedule)

				var waitlist []models.Enrollment
				tx.Where("schedule_id = ? AND status = ?", schedule.ID, "waitlisted").
					Order("waitlist_pos ASC").
					Find(&waitlist)
				for i, w := range waitlist {
					w.WaitlistPos = i + 1
					tx.Save(&w)
				}
			}
		}
	}

	enroll.Status = "cancelled"
	if req.RefundAmount > 0 {
		enroll.RefundAmount = req.RefundAmount
	} else {
		enroll.RefundAmount = enroll.PricePaid
	}
	tx.Save(&enroll)

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"enrollment": enroll})
}

type checkinReq struct {
	UseSession bool `json:"use_session"`
}

func (h *Handler) CheckInStudent(c *gin.Context) {
	var enroll models.Enrollment
	if err := h.DB.First(&enroll, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "报名记录不存在"})
		return
	}
	if enroll.Status != "enrolled" && enroll.Status != "waitlisted" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "该报名状态不可签到"})
		return
	}

	var schedule models.Schedule
	h.DB.First(&schedule, enroll.ScheduleID)

	tx := h.DB.Begin()

	if enroll.Status == "waitlisted" {
		if schedule.Enrolled >= schedule.Capacity {
			tx.Rollback()
			c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "学员在候补名单中，暂无名额"})
			return
		}
		enroll.Status = "enrolled"
		enroll.WaitlistPos = 0
		schedule.Enrolled++
		tx.Save(&schedule)
	}

	if enroll.CheckedIn {
		tx.Rollback()
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "学员已签到"})
		return
	}

	now := time.Now()
	enroll.CheckedIn = true
	enroll.CheckedInAt = &now
	tx.Save(&enroll)

	var course models.Course
	tx.First(&course, schedule.CourseID)

	var req checkinReq
	c.ShouldBindJSON(&req)
	if course.CourseType == "private" || req.UseSession {
		usage := models.SessionUsage{
			StudentID:    enroll.StudentID,
			CourseID:     enroll.CourseID,
			ScheduleID:   enroll.ScheduleID,
			EnrollmentID: enroll.ID,
			SessionsUsed: 1,
			TotalSessions: course.TotalSessions,
		}
		tx.Create(&usage)
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"enrollment": enroll})
}

func (h *Handler) BulkCheckIn(c *gin.Context) {
	var req struct {
		ScheduleID uint   `json:"schedule_id" binding:"required"`
		IDs        []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}

	var schedule models.Schedule
	if err := h.DB.First(&schedule, req.ScheduleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "排课不存在"})
		return
	}

	q := h.DB.Model(&models.Enrollment{}).Where("schedule_id = ?", req.ScheduleID)
	if len(req.IDs) > 0 {
		q = q.Where("id IN ?", req.IDs)
	}
	var ids []uint
	q.Pluck("id", &ids)

	checked := 0
	var errs []string
	now := time.Now()
	for _, id := range ids {
		var enroll models.Enrollment
		h.DB.First(&enroll, id)
		if enroll.CheckedIn || enroll.Status != "enrolled" {
			continue
		}
		enroll.CheckedIn = true
		enroll.CheckedInAt = &now
		if err := h.DB.Save(&enroll).Error; err != nil {
			errs = append(errs, strconv.Itoa(int(id))+":"+err.Error())
			continue
		}
		checked++
	}

	c.JSON(http.StatusOK, gin.H{"checked_in": checked, "errors": errs})
}

func (h *Handler) SessionsByStudent(c *gin.Context) {
	studentID, err := strconv.ParseUint(c.Param("student_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "学员ID不合法"})
		return
	}
	type row struct {
		models.SessionUsage
		CourseName   string `json:"course_name"`
		ScheduleDate string `json:"schedule_date"`
		StartHour    int    `json:"start_hour"`
		EndHour      int    `json:"end_hour"`
	}
	var list []row
	h.DB.Table("session_usages su").
		Select("su.*, c.name as course_name, s.schedule_date, s.start_hour, s.end_hour").
		Joins("LEFT JOIN courses c ON su.course_id = c.id").
		Joins("LEFT JOIN schedules s ON su.schedule_id = s.id").
		Where("su.student_id = ?", uint(studentID)).
		Order("su.id DESC").
		Scan(&list)
	c.JSON(http.StatusOK, list)
}

func (h *Handler) SchedulesByStudent(c *gin.Context) {
	studentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "学员ID不合法"})
		return
	}
	type row struct {
		models.Schedule
		EnrollStatus   string `json:"enroll_status"`
		WaitlistPos    int    `json:"waitlist_pos"`
		CheckedIn      bool   `json:"checked_in"`
		EnrollmentID   uint   `json:"enrollment_id"`
		CourseName     string `json:"course_name"`
		CourseType     string `json:"course_type"`
		CoachName      string `json:"coach_name"`
		VenueName      string `json:"venue_name"`
	}
	var list []row
	h.DB.Table("enrollments e").
		Select(`s.*, e.status as enroll_status, e.waitlist_pos, e.checked_in, e.id as enrollment_id,
			    c.name as course_name, c.course_type, co.name as coach_name, v.name as venue_name`).
		Joins("LEFT JOIN schedules s ON e.schedule_id = s.id").
		Joins("LEFT JOIN courses c ON e.course_id = c.id").
		Joins("LEFT JOIN coaches co ON s.coach_id = co.id").
		Joins("LEFT JOIN venues v ON s.venue_id = v.id").
		Where("e.student_id = ?", uint(studentID)).
		Order("s.schedule_date DESC, s.start_hour ASC").
		Scan(&list)
	c.JSON(http.StatusOK, list)
}

func (h *Handler) ConsumeSession(c *gin.Context) {
	var req struct {
		StudentID uint `json:"student_id" binding:"required"`
		CourseID  uint `json:"course_id" binding:"required"`
		Sessions  int  `json:"sessions" binding:"required"`
		Notes     string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	if req.Sessions <= 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "核销课时数必须为正"})
		return
	}
	usage := models.SessionUsage{
		StudentID:    req.StudentID,
		CourseID:     req.CourseID,
		SessionsUsed: req.Sessions,
		Notes:        req.Notes,
	}
	var course models.Course
	if err := h.DB.First(&course, req.CourseID).Error; err == nil {
		usage.TotalSessions = course.TotalSessions
	}
	h.DB.Create(&usage)
	c.JSON(http.StatusCreated, usage)
}

func (h *Handler) ListSessionUsages(c *gin.Context) {
	var list []models.SessionUsage
	q := h.DB.Order("id DESC")
	if sid := c.Query("student_id"); sid != "" {
		q = q.Where("student_id = ?", sid)
	}
	if cid := c.Query("course_id"); cid != "" {
		q = q.Where("course_id = ?", cid)
	}
	q.Find(&list)
	c.JSON(http.StatusOK, list)
}
