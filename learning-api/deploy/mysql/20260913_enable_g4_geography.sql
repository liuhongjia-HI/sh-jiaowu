-- 线上已有数据库迁移：以管理后台年级学科目录为准，补齐 G4 地理课程范围。
INSERT INTO learning_spaces (id, academic_year, grade, subject, semester, phase, level, name, status)
SELECT REPLACE(id, '-science-', '-geography-'), academic_year, grade, '地理', semester, phase, level,
       REPLACE(name, '科学', '地理'), '启用'
FROM learning_spaces
WHERE grade = '四年级' AND subject IN ('科学', 'Science')
ON DUPLICATE KEY UPDATE status = '启用', subject = VALUES(subject), name = VALUES(name);

UPDATE learning_spaces
SET status = '启用'
WHERE grade = '四年级' AND subject IN ('地理', 'Geography');

-- 若课程表只有历史的 G4 科学课程，也同步复制课程记录，保证学科页有可展示的课程范围。
INSERT INTO courses (id, learning_space_id, name, subject, grade, status, chapter_count, chapters_json)
SELECT REPLACE(c.id, 'science', 'geography'), REPLACE(c.learning_space_id, '-science-', '-geography-'),
       REPLACE(c.name, '科学', '地理'), '地理', c.grade, c.status, c.chapter_count, c.chapters_json
FROM courses c
WHERE c.grade = '四年级' AND c.subject IN ('科学', 'Science')
ON DUPLICATE KEY UPDATE learning_space_id=VALUES(learning_space_id), subject=VALUES(subject), status=VALUES(status);
