package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"venue-booking-admin/internal/auth"
	"venue-booking-admin/internal/config"
	"venue-booking-admin/internal/db"
	"venue-booking-admin/internal/handlers"
	"venue-booking-admin/internal/seed"
)

func main() {
	cfg := config.Load()
	auth.SetSecret(cfg.JWTSecret)

	database, err := db.Connect(cfg.DSN)
	if err != nil {
		log.Fatalf("无法连接数据库: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	if err := seed.Run(database, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		log.Fatalf("种子数据初始化失败: %v", err)
	}

	h := &handlers.Handler{DB: database}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	api := r.Group("/api")
	{
		api.GET("/health", h.Health)
		api.POST("/auth/login", h.Login)

		secured := api.Group("")
		secured.Use(auth.Middleware(database))
		{
			secured.GET("/auth/me", h.Me)

			secured.GET("/venues", h.ListVenues)
			secured.POST("/venues", h.CreateVenue)
			secured.GET("/venues/:id", h.GetVenue)
			secured.PUT("/venues/:id", h.UpdateVenue)
			secured.DELETE("/venues/:id", h.DeleteVenue)

			secured.GET("/bookings", h.ListBookings)
			secured.POST("/bookings", h.CreateBooking)
			secured.PATCH("/bookings/:id/status", h.UpdateBookingStatus)

			secured.GET("/dashboard/stats", h.DashboardStats)

			secured.GET("/coaches", h.ListCoaches)
			secured.POST("/coaches", h.CreateCoach)
			secured.GET("/coaches/:id", h.GetCoach)
			secured.PUT("/coaches/:id", h.UpdateCoach)
			secured.DELETE("/coaches/:id", h.DeleteCoach)
			secured.GET("/coaches/:coach_id/schedules", h.SchedulesByCoach)

			secured.GET("/courses", h.ListCourses)
			secured.POST("/courses", h.CreateCourse)
			secured.GET("/courses/:id", h.GetCourse)
			secured.PUT("/courses/:id", h.UpdateCourse)
			secured.DELETE("/courses/:id", h.DeleteCourse)

			secured.GET("/schedules", h.ListSchedules)
			secured.POST("/schedules", h.CreateSchedule)
			secured.POST("/schedules/validate", h.ValidateSchedule)
			secured.POST("/schedules/batch", h.BatchCreateSchedules)
			secured.POST("/schedules/smart", h.SmartSchedule)
			secured.GET("/schedules/:id", h.GetSchedule)
			secured.PUT("/schedules/:id", h.UpdateSchedule)
			secured.PATCH("/schedules/:id/status", h.UpdateScheduleStatus)
			secured.DELETE("/schedules/:id", h.DeleteSchedule)
			secured.GET("/venues/:venue_id/schedules", h.SchedulesByVenue)
			secured.POST("/schedules/reschedule", h.RescheduleClass)

			secured.GET("/students", h.ListStudents)
			secured.POST("/students", h.CreateStudent)
			secured.GET("/students/:id", h.GetStudent)
			secured.PUT("/students/:id", h.UpdateStudent)
			secured.DELETE("/students/:id", h.DeleteStudent)
			secured.GET("/students/:student_id/schedules", h.SchedulesByStudent)
			secured.GET("/students/:student_id/sessions", h.SessionsByStudent)

			secured.POST("/enrollments", h.EnrollStudent)
			secured.POST("/enrollments/:id/cancel", h.CancelEnrollment)
			secured.POST("/enrollments/:id/checkin", h.CheckInStudent)
			secured.POST("/enrollments/bulk-checkin", h.BulkCheckIn)
			secured.POST("/enrollments/:id/refund", h.ProcessRefund)

			secured.GET("/sessions", h.ListSessionUsages)
			secured.POST("/sessions/consume", h.ConsumeSession)

			secured.GET("/leaves", h.ListLeaves)
			secured.POST("/leaves", h.CreateLeave)
			secured.POST("/leaves/:id/approve", h.ApproveLeave)
			secured.POST("/leaves/:id/reject", h.RejectLeave)

			secured.GET("/schedule-adjustments", h.ListAdjustments)

			secured.GET("/stats/attendance", h.AttendanceRate)
			secured.GET("/stats/coach-workload", h.CoachWorkload)
			secured.GET("/stats/revenue", h.RevenueStats)
			secured.GET("/stats/enrollment-dashboard", h.EnrollmentDashboard)
		}
	}

	log.Printf("venue-booking-admin listening on :%s", cfg.Port)
	if err := r.Run("0.0.0.0:" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
