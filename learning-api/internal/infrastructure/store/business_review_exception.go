package store

import (
	"errors"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"time"
)

func (s *MemoryStore) ReviewExceptionReasons(p learning.Principal) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !canReviewHomework(p) {
		return nil, errors.New("当前账号没有批改权限")
	}
	return append([]string{}, s.bindingForBusinessKind(learning.NoticeReviewException).ApprovedReasons...), nil
}

// Marking an exception never changes a score or fabricates a submission result.
func (s *MemoryStore) MarkReviewException(operator string, p learning.Principal, id string, req learning.ReviewExceptionRequest) (learning.Review, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.Review, error) {
		if !canReviewHomework(p) {
			return learning.Review{}, errors.New("当前账号没有批改权限")
		}
		id = strings.TrimSpace(id)
		req.RequestID = strings.TrimSpace(req.RequestID)
		if req.RequestID == "" || len(req.RequestID) > 128 {
			return learning.Review{}, errors.New("请提供有效的异常操作编号")
		}
		req.Reason = strings.TrimSpace(req.Reason)
		req.ClassName = strings.TrimSpace(req.ClassName)
		if req.ClassName == "" || len([]rune(req.ClassName)) > 20 {
			return learning.Review{}, errors.New("请填写20个字符以内的实际班级名称")
		}
		binding := work.bindingForBusinessKind(learning.NoticeReviewException)
		if req.Reason == "" || !containsString(binding.ApprovedReasons, req.Reason) {
			return learning.Review{}, errors.New("请选择后台配置的微信审核异常原因")
		}
		visible := false
		for _, review := range work.reviewsUnlocked(p) {
			if review.ID == id {
				visible = true
				break
			}
		}
		if !visible {
			return learning.Review{}, errors.New("批改记录不存在或没有权限操作")
		}
		for i, review := range work.reviews {
			if review.ID != id {
				continue
			}
			student, valid := work.findStudent(review.StudentID)
			if !valid || student.AccountStatus != "正常" {
				return learning.Review{}, errors.New("学生账号已失效")
			}
			homework, valid := work.findHomework(review.HomeworkID)
			if !valid {
				return learning.Review{}, errors.New("作业不存在")
			}
			if _, err := work.studentHomeworkUnlocked(learning.Principal{StudentID: student.ID}, homework.ID); err != nil {
				return learning.Review{}, errors.New("作业访问权限已失效")
			}
			if review.Status != "待批改" && review.Status != "待复核" && review.Status != "批改异常" {
				return learning.Review{}, errors.New("该任务当前不可标记异常")
			}
			eventID := businessNoticeHash(learning.NoticeReviewException, id, req.RequestID)
			if previous, found := work.businessEvent(eventID); found {
				if previous.Values["const2"] != req.Reason || previous.Values["thing5"] != req.ClassName {
					return learning.Review{}, errors.New("同一异常操作编号不能修改内容")
				}
				if review.ExceptionEventID != eventID {
					return learning.Review{}, errors.New("该异常操作已处理或解除，请刷新后重新操作")
				}
				return review, nil
			}
			if review.Status == "批改异常" && review.ExceptionReason == req.Reason && review.ExceptionClass == req.ClassName && review.ExceptionEventID != "" {
				return review, nil
			}
			now := time.Now()

			review.Homework = homework.Title
			review.Status = "批改异常"
			review.ExceptionReason = req.Reason
			review.ExceptionClass = req.ClassName
			review.ExceptionEventID = eventID
			work.reviews[i] = review
			values := reviewExceptionValues(review)
			if err := validateOfficialMessageValues(values); err != nil {
				return learning.Review{}, err
			}
			event := learning.BusinessNoticeEvent{ID: eventID, BatchID: eventID, Kind: learning.NoticeReviewException, StudentID: student.ID, StudentName: student.Name, RelatedID: review.ID, Title: "作业批改异常", Summary: homework.Title + " / " + req.ClassName + " / " + req.Reason, Values: values, CreatedAt: businessTime(now), ExpiresAt: businessTime(now.Add(7 * 24 * time.Hour))}
			work.addBusinessNoticeEvent(event, now, now, true)
			work.prependLog(operator, "标记批改异常", student.Name+" · "+homework.Title+" · "+req.ClassName+" · "+req.Reason)
			return review, nil
		}
		return learning.Review{}, errors.New("批改记录不存在")
	}, nil)
}

func reviewExceptionValues(review learning.Review) map[string]string {
	return map[string]string{"thing7": businessShortName(review.Homework), "thing5": review.ExceptionClass, "const2": review.ExceptionReason}
}
func (s *MemoryStore) currentReviewException(event learning.BusinessNoticeEvent) (learning.Review, bool) {
	for _, review := range s.reviews {
		if review.ID == event.RelatedID && review.StudentID == event.StudentID && review.Status == "批改异常" && review.ExceptionEventID == event.ID {
			return review, true
		}
	}
	return learning.Review{}, false
}
