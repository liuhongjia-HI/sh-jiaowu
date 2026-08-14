package learningapp

type Repository interface {
	AuthRepository
	StaffRepository
	StudentRepository
	ContentRepository
	GrantRepository
	SchedulingRepository
	CommercialRepository
	NoticeRepository
	SystemRepository
}

type Service struct {
	auth       AuthRepository
	staff      StaffRepository
	student    StudentRepository
	content    ContentRepository
	grant      GrantRepository
	scheduling SchedulingRepository
	commercial CommercialRepository
	notice     NoticeRepository
	system     SystemRepository
}

func NewService(repo Repository) *Service {
	return &Service{auth: repo, staff: repo, student: repo, content: repo, grant: repo, scheduling: repo, commercial: repo, notice: repo, system: repo}
}
