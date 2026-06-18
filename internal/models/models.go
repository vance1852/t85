package models

import "time"

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex" json:"username"`
	PasswordHash string    `gorm:"size:255" json:"-"`
	DisplayName  string    `gorm:"size:64" json:"display_name"`
	CreatedAt    time.Time `json:"created_at"`
}

type Venue struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128" json:"name"`
	SportType   string    `gorm:"size:32" json:"sport_type"`
	Capacity    int       `json:"capacity"`
	HourlyPrice float64   `json:"hourly_price"`
	OpenHour    int       `json:"open_hour"`
	CloseHour   int       `json:"close_hour"`
	Status      string    `gorm:"size:16" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type Booking struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	VenueID      uint      `gorm:"index" json:"venue_id"`
	CustomerName string    `gorm:"size:64" json:"customer_name"`
	Phone        string    `gorm:"size:32" json:"phone"`
	BookDate     string    `gorm:"size:10;index" json:"book_date"`
	StartHour    int       `json:"start_hour"`
	EndHour      int       `json:"end_hour"`
	Amount       float64   `json:"amount"`
	Status       string    `gorm:"size:16" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type Coach struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Name          string    `gorm:"size:64" json:"name"`
	Phone         string    `gorm:"size:32" json:"phone"`
	Skills        string    `gorm:"type:text" json:"skills"`
	AvailableTime string    `gorm:"type:text" json:"available_time"`
	DailyMaxHours int       `json:"daily_max_hours"`
	WeeklyMaxHours int      `json:"weekly_max_hours"`
	HourlyRate    float64   `json:"hourly_rate"`
	Status        string    `gorm:"size:16" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type Course struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128" json:"name"`
	SportType   string    `gorm:"size:32" json:"sport_type"`
	CourseType  string    `gorm:"size:16" json:"course_type"`
	Duration    int       `json:"duration"`
	Capacity    int       `json:"capacity"`
	Price       float64   `json:"price"`
	TotalSessions int     `json:"total_sessions"`
	Description string    `gorm:"type:text" json:"description"`
	Status      string    `gorm:"size:16" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type Schedule struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CourseID     uint      `gorm:"index" json:"course_id"`
	CoachID      uint      `gorm:"index" json:"coach_id"`
	VenueID      uint      `gorm:"index" json:"venue_id"`
	ScheduleDate string    `gorm:"size:10;index" json:"schedule_date"`
	StartHour    int       `json:"start_hour"`
	EndHour      int       `json:"end_hour"`
	Capacity     int       `json:"capacity"`
	Enrolled     int       `json:"enrolled"`
	Status       string    `gorm:"size:16" json:"status"`
	SessionIndex int       `json:"session_index"`
	Notes        string    `gorm:"type:text" json:"notes"`
	CreatedAt    time.Time `json:"created_at"`

	Course Course `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	Coach  Coach  `gorm:"foreignKey:CoachID" json:"coach,omitempty"`
	Venue  Venue  `gorm:"foreignKey:VenueID" json:"venue,omitempty"`
}

type Student struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64" json:"name"`
	Phone     string    `gorm:"size:32;uniqueIndex" json:"phone"`
	Email     string    `gorm:"size:128" json:"email"`
	Gender    string    `gorm:"size:8" json:"gender"`
	Age       int       `json:"age"`
	Level     string    `gorm:"size:16" json:"level"`
	Notes     string    `gorm:"type:text" json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

type Enrollment struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	StudentID   uint      `gorm:"index:idx_student_schedule,priority:1;index" json:"student_id"`
	ScheduleID  uint      `gorm:"index:idx_student_schedule,priority:2;index" json:"schedule_id"`
	CourseID    uint      `gorm:"index" json:"course_id"`
	EnrollType  string    `gorm:"size:16" json:"enroll_type"`
	Status      string    `gorm:"size:16;index" json:"status"`
	WaitlistPos int       `json:"waitlist_pos"`
	PricePaid   float64   `json:"price_paid"`
	RefundAmount float64  `json:"refund_amount"`
	CheckedIn   bool      `gorm:"default:false" json:"checked_in"`
	CheckedInAt *time.Time `json:"checked_in_at"`
	CreatedAt   time.Time `json:"created_at"`

	Student  Student  `gorm:"foreignKey:StudentID" json:"student,omitempty"`
	Schedule Schedule `gorm:"foreignKey:ScheduleID" json:"schedule,omitempty"`
	Course   Course   `gorm:"foreignKey:CourseID" json:"course,omitempty"`
}

type CoachLeave struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	CoachID     uint      `gorm:"index" json:"coach_id"`
	LeaveDate   string    `gorm:"size:10;index" json:"leave_date"`
	StartHour   int       `json:"start_hour"`
	EndHour     int       `json:"end_hour"`
	Reason      string    `gorm:"type:text" json:"reason"`
	Status      string    `gorm:"size:16" json:"status"`
	Handled     bool      `gorm:"default:false" json:"handled"`
	CreatedAt   time.Time `json:"created_at"`
}

type SessionUsage struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	StudentID    uint      `gorm:"index" json:"student_id"`
	CourseID     uint      `gorm:"index" json:"course_id"`
	ScheduleID   uint      `gorm:"index" json:"schedule_id"`
	EnrollmentID uint      `gorm:"index" json:"enrollment_id"`
	SessionsUsed int       `json:"sessions_used"`
	TotalSessions int      `json:"total_sessions"`
	Notes        string    `gorm:"type:text" json:"notes"`
	CreatedAt    time.Time `json:"created_at"`
}

type ScheduleAdjustment struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	OriginalScheduleID uint `gorm:"index" json:"original_schedule_id"`
	NewScheduleID  *uint     `gorm:"index" json:"new_schedule_id"`
	AdjustType     string    `gorm:"size:16" json:"adjust_type"`
	Reason         string    `gorm:"type:text" json:"reason"`
	HandledBy      string    `gorm:"size:64" json:"handled_by"`
	CreatedAt      time.Time `json:"created_at"`
}
