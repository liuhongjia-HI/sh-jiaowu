package learningapp

import "starline/learning-api/internal/domain/learning"

func (s *Service) PreviewCourseDirectorySync(p learning.Principal, r learning.CourseDirectorySyncRequest) (learning.CourseDirectorySyncResult, error) {
	return s.content.PreviewCourseDirectorySync(p, r)
}
func (s *Service) SyncCourseDirectory(o string, p learning.Principal, r learning.CourseDirectorySyncRequest) (learning.CourseDirectorySyncResult, error) {
	return s.content.SyncCourseDirectory(o, p, r)
}
