package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"venue-booking-admin/internal/models"
)

type courseReq struct {
	Name          string  `json:"name" binding:"required"`
	SportType     string  `json:"sport_type"`
	CourseType    string  `json:"course_type" binding:"required"`
	Duration      int     `json:"duration" binding:"required"`
	Capacity      int     `json:"capacity"`
	Price         float64 `json:"price"`
	TotalSessions int     `json:"total_sessions"`
	Description   string  `json:"description"`
	Status        string  `json:"status"`
}

var validCourseTypes = map[string]bool{"private": true, "small": true, "large": true}

func (h *Handler) ListCourses(c *gin.Context) {
	var courses []models.Course
	q := h.DB.Order("id")
	if name := c.Query("name"); name != "" {
		q = q.Where("name LIKE ?", "%"+name+"%")
	}
	if sport := c.Query("sport_type"); sport != "" {
		q = q.Where("sport_type = ?", sport)
	}
	if ct := c.Query("course_type"); ct != "" {
		q = q.Where("course_type = ?", ct)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	q.Find(&courses)
	c.JSON(http.StatusOK, courses)
}

func (h *Handler) CreateCourse(c *gin.Context) {
	var req courseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	if !validCourseTypes[req.CourseType] {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "课程类型必须为 private/small/large"})
		return
	}
	if req.Duration <= 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "课程时长必须大于0"})
		return
	}
	if req.Capacity <= 0 {
		req.Capacity = 1
	}
	if req.CourseType == "private" && req.Capacity > 1 {
		req.Capacity = 1
	}
	if req.CourseType == "small" && (req.Capacity < 2 || req.Capacity > 10) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "小班课容量应在2-10之间"})
		return
	}
	if req.CourseType == "large" && req.Capacity < 10 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "大课容量应不小于10"})
		return
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	totalSessions := req.TotalSessions
	if totalSessions <= 0 {
		totalSessions = 1
	}
	course := models.Course{
		Name:          req.Name,
		SportType:     req.SportType,
		CourseType:    req.CourseType,
		Duration:      req.Duration,
		Capacity:      req.Capacity,
		Price:         req.Price,
		TotalSessions: totalSessions,
		Description:   req.Description,
		Status:        status,
	}
	h.DB.Create(&course)
	c.JSON(http.StatusCreated, course)
}

func (h *Handler) GetCourse(c *gin.Context) {
	var course models.Course
	if err := h.DB.First(&course, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "课程不存在"})
		return
	}
	c.JSON(http.StatusOK, course)
}

func (h *Handler) UpdateCourse(c *gin.Context) {
	var course models.Course
	if err := h.DB.First(&course, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "课程不存在"})
		return
	}
	var req courseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	course.Name = req.Name
	if req.SportType != "" {
		course.SportType = req.SportType
	}
	if req.CourseType != "" {
		if !validCourseTypes[req.CourseType] {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "课程类型不合法"})
			return
		}
		course.CourseType = req.CourseType
	}
	if req.Duration > 0 {
		course.Duration = req.Duration
	}
	if req.Capacity > 0 {
		course.Capacity = req.Capacity
	}
	if req.Price >= 0 {
		course.Price = req.Price
	}
	if req.TotalSessions > 0 {
		course.TotalSessions = req.TotalSessions
	}
	if req.Description != "" {
		course.Description = req.Description
	}
	if req.Status != "" {
		course.Status = req.Status
	}
	h.DB.Save(&course)
	c.JSON(http.StatusOK, course)
}

func (h *Handler) DeleteCourse(c *gin.Context) {
	var course models.Course
	if err := h.DB.First(&course, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "课程不存在"})
		return
	}
	var count int64
	h.DB.Model(&models.Schedule{}).
		Where("course_id = ? AND status <> ?", course.ID, "cancelled").
		Count(&count)
	if count > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "该课程还有未取消的排课，无法删除"})
		return
	}
	h.DB.Delete(&course)
	c.Status(http.StatusNoContent)
}
