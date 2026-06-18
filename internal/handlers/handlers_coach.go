package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"venue-booking-admin/internal/models"
)

type coachReq struct {
	Name           string  `json:"name" binding:"required"`
	Phone          string  `json:"phone"`
	Skills         string  `json:"skills"`
	AvailableTime  string  `json:"available_time"`
	DailyMaxHours  int     `json:"daily_max_hours"`
	WeeklyMaxHours int     `json:"weekly_max_hours"`
	HourlyRate     float64 `json:"hourly_rate"`
	Status         string  `json:"status"`
}

func (h *Handler) ListCoaches(c *gin.Context) {
	var coaches []models.Coach
	q := h.DB.Order("id")
	if name := c.Query("name"); name != "" {
		q = q.Where("name LIKE ?", "%"+name+"%")
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	if sport := c.Query("sport"); sport != "" {
		q = q.Where("skills LIKE ?", "%"+sport+"%")
	}
	q.Find(&coaches)
	c.JSON(http.StatusOK, coaches)
}

func (h *Handler) CreateCoach(c *gin.Context) {
	var req coachReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	if req.DailyMaxHours < 0 || req.WeeklyMaxHours < 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "课时上限不能为负"})
		return
	}
	if req.WeeklyMaxHours > 0 && req.DailyMaxHours > 0 && req.DailyMaxHours*7 < req.WeeklyMaxHours {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "日课时上限×7不能小于周课时上限"})
		return
	}
	coach := models.Coach{
		Name:           req.Name,
		Phone:          req.Phone,
		Skills:         req.Skills,
		AvailableTime:  req.AvailableTime,
		DailyMaxHours:  req.DailyMaxHours,
		WeeklyMaxHours: req.WeeklyMaxHours,
		HourlyRate:     req.HourlyRate,
		Status:         status,
	}
	h.DB.Create(&coach)
	c.JSON(http.StatusCreated, coach)
}

func (h *Handler) GetCoach(c *gin.Context) {
	var coach models.Coach
	if err := h.DB.First(&coach, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "教练不存在"})
		return
	}
	c.JSON(http.StatusOK, coach)
}

func (h *Handler) UpdateCoach(c *gin.Context) {
	var coach models.Coach
	if err := h.DB.First(&coach, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "教练不存在"})
		return
	}
	var req coachReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	coach.Name = req.Name
	if req.Phone != "" {
		coach.Phone = req.Phone
	}
	if req.Skills != "" {
		coach.Skills = req.Skills
	}
	if req.AvailableTime != "" {
		coach.AvailableTime = req.AvailableTime
	}
	if req.DailyMaxHours >= 0 {
		coach.DailyMaxHours = req.DailyMaxHours
	}
	if req.WeeklyMaxHours >= 0 {
		coach.WeeklyMaxHours = req.WeeklyMaxHours
	}
	if req.HourlyRate >= 0 {
		coach.HourlyRate = req.HourlyRate
	}
	if req.Status != "" {
		coach.Status = req.Status
	}
	h.DB.Save(&coach)
	c.JSON(http.StatusOK, coach)
}

func (h *Handler) DeleteCoach(c *gin.Context) {
	var coach models.Coach
	if err := h.DB.First(&coach, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "教练不存在"})
		return
	}
	var count int64
	h.DB.Model(&models.Schedule{}).
		Where("coach_id = ? AND status <> ?", coach.ID, "cancelled").
		Count(&count)
	if count > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "该教练还有未取消的排课，无法删除"})
		return
	}
	h.DB.Delete(&coach)
	c.Status(http.StatusNoContent)
}
