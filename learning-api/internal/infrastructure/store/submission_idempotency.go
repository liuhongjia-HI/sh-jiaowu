package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// The named lock covers both the authoritative lookup and the transaction.
// A local mutex alone cannot protect retries reaching different API instances.
func (s *MemoryStore) lockSubmissionRequest(studentID, requestID string) (func(), error) {
	if s.db == nil || requestID == "" {
		return func() {}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	name := "submission:" + businessNoticeHash(studentID, requestID)[:40]
	var acquired sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 5)", name).Scan(&acquired); err != nil || !acquired.Valid || acquired.Int64 != 1 {
		conn.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("该提交正在保存，请稍后重试")
	}
	release := func() {
		releaseCtx, done := context.WithTimeout(context.Background(), 2*time.Second)
		defer done()
		conn.ExecContext(releaseCtx, "SELECT RELEASE_LOCK(?)", name)
		conn.Close()
	}
	var item learning.Submission
	var answers string
	var createdAt sql.NullTime
	err = conn.QueryRowContext(ctx, `SELECT id, homework_id, student_id, task_title, score, objective_score, final_score, teacher_comment, reward, status, answers_json, created_at FROM student_submission_results WHERE student_id=? AND request_id=?`, studentID, requestID).Scan(&item.ID, &item.HomeworkID, &item.StudentID, &item.TaskTitle, &item.Score, &item.ObjectiveScore, &item.FinalScore, &item.TeacherComment, &item.Reward, &item.Status, &answers, &createdAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		release()
		return nil, err
	}
	if err == nil {
		item.RequestID = requestID
		item.Answers = parseSubmissionAnswersJSON(answers)
		item.CreatedAt = dateTimeString(createdAt)
		s.submissions[item.ID] = item
	}
	return release, nil
}
