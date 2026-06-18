package seed

import (
	"log"

	"gorm.io/gorm"

	"venue-booking-admin/internal/auth"
	"venue-booking-admin/internal/models"
)

func Run(database *gorm.DB, adminUser, adminPass string) error {
	var count int64
	database.Model(&models.User{}).Where("username = ?", adminUser).Count(&count)
	if count == 0 {
		hash, err := auth.HashPassword(adminPass)
		if err != nil {
			return err
		}
		database.Create(&models.User{Username: adminUser, PasswordHash: hash, DisplayName: "平台管理员"})
		log.Println("已创建管理员账号")
	}

	var venueCount int64
	database.Model(&models.Venue{}).Count(&venueCount)
	venues := []models.Venue{
		{Name: "城北全民健身中心篮球馆", SportType: "basketball", Capacity: 200, HourlyPrice: 160, OpenHour: 8, CloseHour: 22, Status: "open"},
		{Name: "奥体中心游泳馆", SportType: "swimming", Capacity: 400, HourlyPrice: 80, OpenHour: 6, CloseHour: 21, Status: "open"},
		{Name: "市民广场羽毛球馆", SportType: "badminton", Capacity: 60, HourlyPrice: 50, OpenHour: 9, CloseHour: 22, Status: "open"},
		{Name: "滨江足球公园", SportType: "football", Capacity: 500, HourlyPrice: 300, OpenHour: 8, CloseHour: 20, Status: "open"},
	}
	if venueCount == 0 {
		if err := database.Create(&venues).Error; err != nil {
			return err
		}
		bookings := []models.Booking{
			{VenueID: venues[0].ID, CustomerName: "陈刚", Phone: "13700001111", BookDate: "2026-06-20", StartHour: 18, EndHour: 20, Amount: 320, Status: "booked"},
			{VenueID: venues[0].ID, CustomerName: "周敏", Phone: "13700002222", BookDate: "2026-06-20", StartHour: 20, EndHour: 21, Amount: 160, Status: "booked"},
			{VenueID: venues[1].ID, CustomerName: "黄磊", Phone: "13700003333", BookDate: "2026-06-21", StartHour: 7, EndHour: 9, Amount: 160, Status: "completed"},
			{VenueID: venues[3].ID, CustomerName: "吴静", Phone: "13700004444", BookDate: "2026-06-22", StartHour: 15, EndHour: 17, Amount: 600, Status: "cancelled"},
		}
		if err := database.Create(&bookings).Error; err != nil {
			return err
		}
		log.Println("已初始化场馆与预订种子数据")
	} else {
		database.Find(&venues)
	}

	var coachCount int64
	database.Model(&models.Coach{}).Count(&coachCount)
	coaches := []models.Coach{}
	if coachCount == 0 {
		coaches = []models.Coach{
			{Name: "李建华", Phone: "13800001001", Skills: "basketball,fitness", AvailableTime: "周一至周五 9:00-18:00, 周六日 8:00-20:00", DailyMaxHours: 8, WeeklyMaxHours: 40, HourlyRate: 300, Status: "active"},
			{Name: "王晓燕", Phone: "13800001002", Skills: "swimming,aquafit", AvailableTime: "每日 6:00-21:00", DailyMaxHours: 6, WeeklyMaxHours: 35, HourlyRate: 250, Status: "active"},
			{Name: "张伟", Phone: "13800001003", Skills: "badminton,tennis", AvailableTime: "周二至周日 9:00-22:00", DailyMaxHours: 8, WeeklyMaxHours: 45, HourlyRate: 200, Status: "active"},
			{Name: "刘强", Phone: "13800001004", Skills: "football,soccer", AvailableTime: "每日 8:00-20:00", DailyMaxHours: 10, WeeklyMaxHours: 50, HourlyRate: 350, Status: "active"},
			{Name: "陈美琳", Phone: "13800001005", Skills: "basketball,yoga,fitness", AvailableTime: "周一至周六 10:00-20:00", DailyMaxHours: 6, WeeklyMaxHours: 30, HourlyRate: 280, Status: "active"},
			{Name: "赵鹏飞", Phone: "13800001006", Skills: "swimming", AvailableTime: "周三至周日 8:00-17:00", DailyMaxHours: 7, WeeklyMaxHours: 35, HourlyRate: 220, Status: "active"},
		}
		if err := database.Create(&coaches).Error; err != nil {
			return err
		}
		log.Println("已初始化教练种子数据")
	} else {
		database.Find(&coaches)
	}

	var courseCount int64
	database.Model(&models.Course{}).Count(&courseCount)
	courses := []models.Course{}
	if courseCount == 0 {
		courses = []models.Course{
			{Name: "篮球私教课（成人）", SportType: "basketball", CourseType: "private", Duration: 1, Capacity: 1, Price: 380, TotalSessions: 12, Description: "一对一成人篮球技术私教", Status: "active"},
			{Name: "青少年篮球基础班", SportType: "basketball", CourseType: "small", Duration: 2, Capacity: 8, Price: 220, TotalSessions: 16, Description: "10-16岁青少年篮球基础训练", Status: "active"},
			{Name: "游泳零基础入门班", SportType: "swimming", CourseType: "small", Duration: 1, Capacity: 6, Price: 180, TotalSessions: 10, Description: "蛙泳零基础教学", Status: "active"},
			{Name: "游泳进阶私教", SportType: "swimming", CourseType: "private", Duration: 1, Capacity: 1, Price: 320, TotalSessions: 8, Description: "自由泳/仰泳进阶一对一", Status: "active"},
			{Name: "羽毛球技术小班", SportType: "badminton", CourseType: "small", Duration: 2, Capacity: 6, Price: 160, TotalSessions: 12, Description: "羽毛球技术提升训练", Status: "active"},
			{Name: "羽毛球私教课", SportType: "badminton", CourseType: "private", Duration: 1, Capacity: 1, Price: 260, TotalSessions: 10, Description: "一对一羽毛球技术指导", Status: "active"},
			{Name: "少儿足球启蒙班", SportType: "football", CourseType: "large", Duration: 2, Capacity: 20, Price: 150, TotalSessions: 20, Description: "6-12岁足球基础训练", Status: "active"},
			{Name: "篮球公开大课", SportType: "basketball", CourseType: "large", Duration: 2, Capacity: 30, Price: 120, TotalSessions: 8, Description: "大众篮球健身课", Status: "active"},
		}
		if err := database.Create(&courses).Error; err != nil {
			return err
		}
		log.Println("已初始化课程种子数据")
	} else {
		database.Find(&courses)
	}

	var scheduleCount int64
	database.Model(&models.Schedule{}).Count(&scheduleCount)
	if scheduleCount == 0 {
		schedules := []models.Schedule{
			{CourseID: courses[1].ID, CoachID: coaches[0].ID, VenueID: venues[0].ID, ScheduleDate: "2026-06-23", StartHour: 10, EndHour: 12, Capacity: 8, Enrolled: 5, Status: "scheduled", SessionIndex: 1},
			{CourseID: courses[0].ID, CoachID: coaches[0].ID, VenueID: venues[0].ID, ScheduleDate: "2026-06-23", StartHour: 14, EndHour: 15, Capacity: 1, Enrolled: 1, Status: "scheduled", SessionIndex: 1},
			{CourseID: courses[2].ID, CoachID: coaches[1].ID, VenueID: venues[1].ID, ScheduleDate: "2026-06-23", StartHour: 8, EndHour: 9, Capacity: 6, Enrolled: 6, Status: "scheduled", SessionIndex: 1},
			{CourseID: courses[3].ID, CoachID: coaches[5].ID, VenueID: venues[1].ID, ScheduleDate: "2026-06-23", StartHour: 10, EndHour: 11, Capacity: 1, Enrolled: 0, Status: "scheduled", SessionIndex: 1},
			{CourseID: courses[4].ID, CoachID: coaches[2].ID, VenueID: venues[2].ID, ScheduleDate: "2026-06-23", StartHour: 15, EndHour: 17, Capacity: 6, Enrolled: 4, Status: "scheduled", SessionIndex: 1},
			{CourseID: courses[6].ID, CoachID: coaches[3].ID, VenueID: venues[3].ID, ScheduleDate: "2026-06-24", StartHour: 9, EndHour: 11, Capacity: 20, Enrolled: 15, Status: "scheduled", SessionIndex: 1},
			{CourseID: courses[7].ID, CoachID: coaches[4].ID, VenueID: venues[0].ID, ScheduleDate: "2026-06-24", StartHour: 19, EndHour: 21, Capacity: 30, Enrolled: 18, Status: "scheduled", SessionIndex: 1},
			{CourseID: courses[1].ID, CoachID: coaches[0].ID, VenueID: venues[0].ID, ScheduleDate: "2026-06-25", StartHour: 10, EndHour: 12, Capacity: 8, Enrolled: 3, Status: "scheduled", SessionIndex: 2},
			{CourseID: courses[2].ID, CoachID: coaches[1].ID, VenueID: venues[1].ID, ScheduleDate: "2026-06-25", StartHour: 8, EndHour: 9, Capacity: 6, Enrolled: 5, Status: "scheduled", SessionIndex: 2},
			{CourseID: courses[5].ID, CoachID: coaches[2].ID, VenueID: venues[2].ID, ScheduleDate: "2026-06-25", StartHour: 19, EndHour: 20, Capacity: 1, Enrolled: 1, Status: "scheduled", SessionIndex: 1},
			{CourseID: courses[1].ID, CoachID: coaches[0].ID, VenueID: venues[0].ID, ScheduleDate: "2026-06-21", StartHour: 10, EndHour: 12, Capacity: 8, Enrolled: 6, Status: "completed", SessionIndex: 0},
			{CourseID: courses[4].ID, CoachID: coaches[2].ID, VenueID: venues[2].ID, ScheduleDate: "2026-06-21", StartHour: 15, EndHour: 17, Capacity: 6, Enrolled: 4, Status: "completed", SessionIndex: 0},
		}
		if err := database.Create(&schedules).Error; err != nil {
			return err
		}
		log.Println("已初始化排课种子数据")

		var studentCount int64
		database.Model(&models.Student{}).Count(&studentCount)
		if studentCount == 0 {
			students := []models.Student{
				{Name: "张小明", Phone: "13900002001", Email: "zhangxm@example.com", Gender: "male", Age: 14, Level: "intermediate", Notes: "右手投篮"},
				{Name: "李思思", Phone: "13900002002", Email: "liss@example.com", Gender: "female", Age: 12, Level: "beginner"},
				{Name: "王大雷", Phone: "13900002003", Email: "wangdl@example.com", Gender: "male", Age: 28, Level: "advanced", Notes: "私教学员"},
				{Name: "赵乐乐", Phone: "13900002004", Email: "zhaoll@example.com", Gender: "female", Age: 9, Level: "beginner"},
				{Name: "陈浩南", Phone: "13900002005", Gender: "male", Age: 15, Level: "intermediate"},
				{Name: "刘诗涵", Phone: "13900002006", Email: "liush@example.com", Gender: "female", Age: 11, Level: "beginner"},
				{Name: "孙浩然", Phone: "13900002007", Gender: "male", Age: 35, Level: "advanced", Notes: "成人私教"},
				{Name: "周小宝", Phone: "13900002008", Gender: "male", Age: 8, Level: "beginner"},
				{Name: "吴佳怡", Phone: "13900002009", Email: "wujy@example.com", Gender: "female", Age: 13, Level: "intermediate"},
				{Name: "郑小龙", Phone: "13900002010", Gender: "male", Age: 10, Level: "beginner"},
				{Name: "冯雨桐", Phone: "13900002011", Gender: "female", Age: 16, Level: "advanced"},
				{Name: "黄子豪", Phone: "13900002012", Gender: "male", Age: 32, Level: "intermediate"},
			}
			if err := database.Create(&students).Error; err != nil {
				return err
			}
			log.Println("已初始化学员种子数据")

			enrollments := []models.Enrollment{
				{StudentID: students[0].ID, ScheduleID: schedules[0].ID, CourseID: courses[1].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 220, CheckedIn: false},
				{StudentID: students[1].ID, ScheduleID: schedules[0].ID, CourseID: courses[1].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 220, CheckedIn: false},
				{StudentID: students[4].ID, ScheduleID: schedules[0].ID, CourseID: courses[1].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 220, CheckedIn: false},
				{StudentID: students[5].ID, ScheduleID: schedules[0].ID, CourseID: courses[1].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 220, CheckedIn: false},
				{StudentID: students[9].ID, ScheduleID: schedules[0].ID, CourseID: courses[1].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 220, CheckedIn: false},
				{StudentID: students[2].ID, ScheduleID: schedules[1].ID, CourseID: courses[0].ID, EnrollType: "private", Status: "enrolled", WaitlistPos: 0, PricePaid: 380, CheckedIn: false},
				{StudentID: students[3].ID, ScheduleID: schedules[2].ID, CourseID: courses[2].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 180, CheckedIn: false},
				{StudentID: students[5].ID, ScheduleID: schedules[2].ID, CourseID: courses[2].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 180, CheckedIn: false},
				{StudentID: students[7].ID, ScheduleID: schedules[2].ID, CourseID: courses[2].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 180, CheckedIn: false},
				{StudentID: students[8].ID, ScheduleID: schedules[2].ID, CourseID: courses[2].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 180, CheckedIn: false},
				{StudentID: students[10].ID, ScheduleID: schedules[2].ID, CourseID: courses[2].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 180, CheckedIn: false},
				{StudentID: students[0].ID, ScheduleID: schedules[2].ID, CourseID: courses[2].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 180, CheckedIn: false},
				{StudentID: students[4].ID, ScheduleID: schedules[2].ID, CourseID: courses[2].ID, EnrollType: "waitlist", Status: "waitlisted", WaitlistPos: 1, PricePaid: 0},
				{StudentID: students[0].ID, ScheduleID: schedules[4].ID, CourseID: courses[4].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 160, CheckedIn: false},
				{StudentID: students[10].ID, ScheduleID: schedules[4].ID, CourseID: courses[4].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 160, CheckedIn: false},
				{StudentID: students[11].ID, ScheduleID: schedules[4].ID, CourseID: courses[4].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 160, CheckedIn: false},
				{StudentID: students[8].ID, ScheduleID: schedules[4].ID, CourseID: courses[4].ID, EnrollType: "small", Status: "enrolled", WaitlistPos: 0, PricePaid: 160, CheckedIn: false},
				{StudentID: students[3].ID, ScheduleID: schedules[5].ID, CourseID: courses[6].ID, EnrollType: "large", Status: "enrolled", WaitlistPos: 0, PricePaid: 150, CheckedIn: false},
				{StudentID: students[7].ID, ScheduleID: schedules[5].ID, CourseID: courses[6].ID, EnrollType: "large", Status: "enrolled", WaitlistPos: 0, PricePaid: 150, CheckedIn: false},
				{StudentID: students[9].ID, ScheduleID: schedules[5].ID, CourseID: courses[6].ID, EnrollType: "large", Status: "enrolled", WaitlistPos: 0, PricePaid: 150, CheckedIn: false},
			}
			for i := 0; i < 12 && 19+i < len(enrollments)+12; i++ {
				sid := uint(students[i%len(students)].ID)
				if i+19 < len(enrollments) {
					continue
				}
				_ = sid
			}
			if err := database.Create(&enrollments).Error; err != nil {
				return err
			}

			for i := 0; i < 12; i++ {
				idx := 19 + i
				if idx >= len(enrollments) {
					e := models.Enrollment{
						StudentID:  uint(students[i%len(students)].ID),
						ScheduleID: schedules[5].ID,
						CourseID:   courses[6].ID,
						EnrollType: "large",
						Status:     "enrolled",
						PricePaid:  150,
					}
					database.Create(&e)
				}
			}
			for i := 0; i < 18; i++ {
				e := models.Enrollment{
					StudentID:  uint(students[i%len(students)].ID),
					ScheduleID: schedules[6].ID,
					CourseID:   courses[7].ID,
					EnrollType: "large",
					Status:     "enrolled",
					PricePaid:  120,
				}
				database.Create(&e)
			}
			log.Println("已初始化报名种子数据")
		}

		leaves := []models.CoachLeave{
			{CoachID: coaches[1].ID, LeaveDate: "2026-06-26", StartHour: 8, EndHour: 18, Reason: "年度体检", Status: "pending", Handled: false},
		}
		database.Create(&leaves)
		log.Println("已初始化请假种子数据")
	}

	return nil
}
