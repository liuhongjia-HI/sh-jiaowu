package store

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

const materialBatchRegistryKind = "material_upload_batch"

func (s *MemoryStore) PendingMaterialNoticeBatches(p learning.Principal) []learning.PendingMaterialNoticeBatch {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []learning.PendingMaterialNoticeBatch{}
	if !canUploadHandout(p) {
		return rows
	}
	for _, event := range s.businessNoticeEvents {
		if event.Kind != materialBatchRegistryKind || (event.BatchCompleted && event.CompletedResourceCount == len(event.ResourceIDs)) || event.UploaderID != p.UserID {
			continue
		}
		course, ok := s.findCourse(event.CourseID)
		if ok && canSeeCourse(p, course) {
			rows = append(rows, learning.PendingMaterialNoticeBatch{BatchID: event.BatchID, CourseID: event.CourseID, ResourceCount: len(event.ResourceIDs)})
		}
	}
	return rows
}

func materialBatchRegistryID(userID, batchID, courseID string) string {
	return businessNoticeHash(materialBatchRegistryKind, userID, batchID, courseID)
}

// Registry and successful material are committed together. Client completion
// cannot add IDs, notify another uploader's batch, or include failed uploads.
func (s *MemoryStore) registerMaterialNoticeBatch(p learning.Principal, batchID string, item learning.Material) {
	legacy := batchID == ""
	if legacy {
		batchID = item.ID
	}
	id := materialBatchRegistryID(p.UserID, batchID, item.CourseID)
	found := false
	for i := range s.businessNoticeEvents {
		if s.businessNoticeEvents[i].ID == id {
			s.businessNoticeEvents[i].ResourceIDs = appendUnique(s.businessNoticeEvents[i].ResourceIDs, item.ID)
			found = true
			break
		}
	}
	if !found {
		s.businessNoticeEvents = append(s.businessNoticeEvents, learning.BusinessNoticeEvent{ID: id, BatchID: batchID, Kind: materialBatchRegistryKind, UploaderID: p.UserID, CourseID: item.CourseID, ResourceIDs: []string{item.ID}, CreatedAt: businessTime(time.Now())})
	}
	if legacy {
		_, _ = s.completeMaterialNoticeBatchUnlocked(p, batchID, item.CourseID)
	}
}

func (s *MemoryStore) CompleteMaterialNoticeBatch(operator string, p learning.Principal, batchID string, req learning.MaterialNoticeBatchRequest) (learning.MaterialNoticeBatchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) (learning.MaterialNoticeBatchResult, error) {
		result, err := work.completeMaterialNoticeBatchUnlocked(p, strings.TrimSpace(batchID), strings.TrimSpace(req.CourseID))
		if err == nil && !result.AlreadyCompleted {
			work.prependLog(operator, "汇总资料通知", req.CourseID)
		}
		return result, err
	})
}

func (s *MemoryStore) materialBatchAccessibleResources(studentID string, ids []string) []learning.Material {
	items := []learning.Material{}
	for _, id := range ids {
		visible := false
		for _, material := range s.materials {
			if material.ID == id {
				visible = materialVisibleToStudents(material)
				break
			}
		}
		if !visible {
			continue
		}
		if item, err := s.studentMaterialUnlocked(learning.Principal{StudentID: studentID}, id); err == nil {
			items = append(items, item)
		}
	}
	return items
}

func materialBatchValues(binding learning.BusinessNoticeBinding, event learning.BusinessNoticeEvent, items []learning.Material) map[string]string {
	values := map[string]string{}
	if len(items) == 0 {
		return values
	}
	for key, source := range binding.FieldMappings {
		switch source {
		case "course_name":
			values[key] = businessShortName(items[0].Course)
		case "resource_title":
			values[key] = businessShortName(items[0].Title)
		case "student_name":
			values[key] = businessShortName(event.StudentName)
		case "resource_count":
			values[key] = fmt.Sprint(len(items))
		case "published_at":
			format := "2006-01-02 15:04:05"
			if strings.HasPrefix(key, "date") {
				format = "2006-01-02"
			}
			values[key] = parseBusinessTime(event.CreatedAt).In(businessNoticeLocation).Format(format)
		}
	}
	return values
}

func (s *MemoryStore) completeMaterialNoticeBatchUnlocked(p learning.Principal, batchID, courseID string) (learning.MaterialNoticeBatchResult, error) {
	result := learning.MaterialNoticeBatchResult{}
	if batchID == "" || len(batchID) > 64 || !canUploadHandout(p) {
		return result, errors.New("没有权限汇总此上传批次")
	}
	course, ok := s.findCourse(courseID)
	if !ok || !canSeeCourse(p, course) {
		return result, errors.New("课程不存在或无权操作")
	}
	registryID := materialBatchRegistryID(p.UserID, batchID, courseID)
	registryIndex := -1
	for i, event := range s.businessNoticeEvents {
		if event.ID == registryID && event.Kind == materialBatchRegistryKind {
			registryIndex = i
			break
		}
	}
	if registryIndex < 0 {
		return result, errors.New("上传批次不存在或不属于当前账号")
	}
	registry := s.businessNoticeEvents[registryIndex]
	result.ResourceCount, result.AlreadyCompleted = len(registry.ResourceIDs), registry.BatchCompleted
	now := time.Now()
	binding := s.bindingForBusinessKind(learning.NoticeMaterialsPublished)
	for _, student := range s.students {
		if student.AccountStatus != "正常" {
			continue
		}
		items := s.materialBatchAccessibleResources(student.ID, registry.ResourceIDs)
		if len(items) == 0 {
			continue
		}
		ids := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		id := businessNoticeHash(registryID, student.ID)
		event := learning.BusinessNoticeEvent{ID: id, BatchID: registryID, Kind: learning.NoticeMaterialsPublished, StudentID: student.ID, StudentName: student.Name, CourseID: courseID, RelatedID: courseID, ResourceIDs: ids, Title: "资料已更新", Summary: fmt.Sprintf("%s / 共%d份资料", course.Name, len(items)), CreatedAt: registry.CreatedAt, ExpiresAt: businessTime(parseBusinessTime(registry.CreatedAt).Add(7 * 24 * time.Hour))}
		stationID := "notice-upload-" + businessNoticeHash(p.UserID, batchID, courseID, student.ID)
		legacy := len(registry.ResourceIDs) == 1 && batchID == registry.ResourceIDs[0]
		for _, notice := range s.notices {
			if notice.RelatedType == "course" && notice.RelatedID == courseID && notice.RecipientStudentID == student.ID && (notice.ID == stationID || legacy) {
				event.StationNoticeID = notice.ID
				break
			}
		}
		event.Values = materialBatchValues(binding, event, items)
		updated := false
		for i, old := range s.businessNoticeEvents {
			if old.ID == id {
				s.businessNoticeEvents[i] = event
				// A completed operation never creates another external task. Only
				// unclaimed tasks receive counts for files recovered before send.
				for j := range s.businessNoticeTasks {
					if s.businessNoticeTasks[j].EventID == id && s.businessNoticeTasks[j].Status == "待发送" {
						s.businessNoticeTasks[j].Values = cloneMap(event.Values)
					}
				}
				updated = true
				break
			}
		}
		if !updated {
			if registry.BatchCompleted {
				continue
			}
			// Disabled batches are recorded without retroactively replaying them
			// when configuration is enabled later.
			s.addBusinessNoticeEvent(event, now, now, false)
		}
		result.RecipientCount++
	}
	s.businessNoticeEvents[registryIndex].BatchCompleted = true
	s.businessNoticeEvents[registryIndex].CompletedResourceCount = len(registry.ResourceIDs)
	return result, nil
}
