CREATE TABLE IF NOT EXISTS students (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  nickname VARCHAR(64) NOT NULL DEFAULT '',
  avatar_url TEXT NOT NULL,
  grade VARCHAR(32) NOT NULL,
  phone VARCHAR(32) NOT NULL,
  school_name VARCHAR(64) NOT NULL DEFAULT '',
  guardian_name VARCHAR(32) NOT NULL DEFAULT '',
  official_account_open_id VARCHAR(128) NOT NULL DEFAULT '',
  account_status VARCHAR(32) NOT NULL DEFAULT '正常',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS users (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  phone VARCHAR(32) NOT NULL,
  open_id VARCHAR(128) NOT NULL DEFAULT '',
  union_id VARCHAR(128) DEFAULT '',
  account_status VARCHAR(32) NOT NULL DEFAULT '正常',
  remark VARCHAR(255) NOT NULL DEFAULT '',
  student_id VARCHAR(64) DEFAULT '',
  campus_id VARCHAR(64) DEFAULT '',
  password_hash TEXT NOT NULL,
  must_change_password TINYINT(1) NOT NULL DEFAULT 0,
  token_version INT NOT NULL DEFAULT 0,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_users_open_id (open_id),
  KEY idx_users_student_id (student_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS student_subscriptions (
  student_id VARCHAR(64) PRIMARY KEY,
  enabled TINYINT(1) NOT NULL DEFAULT 0,
  template_ids_json TEXT NOT NULL,
  updated_at DATETIME NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS roles (
  code VARCHAR(32) PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  description VARCHAR(255) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_roles (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id VARCHAR(64) NOT NULL,
  role_code VARCHAR(32) NOT NULL,
  UNIQUE KEY uk_user_role (user_id, role_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS teacher_course_scopes (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id VARCHAR(64) NOT NULL,
  course_id VARCHAR(64) NOT NULL,
  UNIQUE KEY uk_teacher_course (user_id, course_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS teacher_class_scopes (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id VARCHAR(64) NOT NULL,
  class_name VARCHAR(64) NOT NULL,
  UNIQUE KEY uk_teacher_class (user_id, class_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS teacher_learning_space_access (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  teacher_id VARCHAR(64) NOT NULL,
  learning_space_id VARCHAR(64) NOT NULL,
  can_view TINYINT(1) NOT NULL DEFAULT 1,
  can_upload_handout TINYINT(1) NOT NULL DEFAULT 0,
  can_upload_question TINYINT(1) NOT NULL DEFAULT 0,
  can_review TINYINT(1) NOT NULL DEFAULT 0,
  can_manage_content TINYINT(1) NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_teacher_learning_space (teacher_id, learning_space_id),
  KEY idx_teacher_learning_space (learning_space_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS admin_campus_scopes (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id VARCHAR(64) NOT NULL,
  campus_id VARCHAR(64) NOT NULL,
  UNIQUE KEY uk_admin_campus (user_id, campus_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS study_packages (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  academic_year VARCHAR(32) NOT NULL,
  grade VARCHAR(32) NOT NULL,
  semester VARCHAR(32) NOT NULL,
  subject VARCHAR(32) NOT NULL,
  phase_scope VARCHAR(32) NOT NULL DEFAULT '全学期',
  package_type VARCHAR(32) NOT NULL DEFAULT 'full',
  sale_starts_at DATE NULL,
  sale_ends_at DATE NULL,
  status VARCHAR(32) NOT NULL DEFAULT '草稿',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS subjects (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT '启用',
  UNIQUE KEY uk_subject_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS learning_spaces (
  id VARCHAR(64) PRIMARY KEY,
  academic_year VARCHAR(32) NOT NULL,
  grade VARCHAR(32) NOT NULL,
  subject VARCHAR(32) NOT NULL,
  semester VARCHAR(32) NOT NULL,
  phase VARCHAR(32) NOT NULL,
  name VARCHAR(128) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT '启用',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_learning_space (academic_year, grade, subject, semester, phase),
  KEY idx_learning_spaces_scope (grade, subject, semester, phase)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS courses (
  id VARCHAR(64) PRIMARY KEY,
  learning_space_id VARCHAR(64) NOT NULL DEFAULT '',
  name VARCHAR(128) NOT NULL,
  subject VARCHAR(32) NOT NULL,
  grade VARCHAR(32) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT '启用',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS package_course_relations (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  package_id VARCHAR(64) NOT NULL,
  course_id VARCHAR(64) NOT NULL,
  UNIQUE KEY uk_package_course (package_id, course_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS package_spaces (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  package_id VARCHAR(64) NOT NULL,
  learning_space_id VARCHAR(64) NOT NULL,
  UNIQUE KEY uk_package_space (package_id, learning_space_id),
  KEY idx_package_spaces_space (learning_space_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS package_content_types (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  package_id VARCHAR(64) NOT NULL,
  content_type VARCHAR(32) NOT NULL,
  UNIQUE KEY uk_package_content_type (package_id, content_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS learning_contents (
  id VARCHAR(64) PRIMARY KEY,
  learning_space_id VARCHAR(64) NOT NULL,
  content_type VARCHAR(32) NOT NULL,
  source_table VARCHAR(64) NOT NULL DEFAULT '',
  source_id VARCHAR(64) NOT NULL DEFAULT '',
  title VARCHAR(128) NOT NULL,
  chapter_id VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT '草稿',
  sort_order INT NOT NULL DEFAULT 0,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_learning_contents_space_type (learning_space_id, content_type, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS student_package_grants (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  student_id VARCHAR(64) NOT NULL,
  package_id VARCHAR(64) NOT NULL,
  starts_at DATE NOT NULL,
  ends_at DATE NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  operator_id VARCHAR(64) NOT NULL DEFAULT '',
  operator_name VARCHAR(64) NOT NULL DEFAULT '',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_student_package (student_id, package_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS student_learning_space_access (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  student_id VARCHAR(64) NOT NULL,
  learning_space_id VARCHAR(64) NOT NULL,
  package_grant_id BIGINT NOT NULL,
  starts_at DATE NOT NULL,
  ends_at DATE NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_student_space_grant (student_id, learning_space_id, package_grant_id),
  KEY idx_student_learning_space (student_id, learning_space_id),
  KEY idx_learning_space_access_status (status, ends_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS availability_slots (
  id VARCHAR(64) PRIMARY KEY,
  owner_type VARCHAR(16) NOT NULL,
  owner_id VARCHAR(64) NOT NULL,
  owner_name VARCHAR(64) NOT NULL DEFAULT '',
  day_of_week TINYINT NOT NULL,
  start_time CHAR(5) NOT NULL,
  end_time CHAR(5) NOT NULL,
  start_date DATE NULL,
  end_date DATE NULL,
  remark VARCHAR(255) NOT NULL DEFAULT '',
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_availability_owner (owner_type, owner_id),
  KEY idx_availability_day (day_of_week, start_time, end_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS schedule_classes (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  course_id VARCHAR(64) NOT NULL,
  course_name VARCHAR(128) NOT NULL DEFAULT '',
  teacher_id VARCHAR(64) NOT NULL,
  teacher_name VARCHAR(64) NOT NULL DEFAULT '',
  campus_id VARCHAR(64) NOT NULL DEFAULT '',
  room_name VARCHAR(64) NOT NULL DEFAULT '',
  class_type VARCHAR(16) NOT NULL,
  capacity INT NOT NULL DEFAULT 1,
  duration_minutes INT NOT NULL DEFAULT 90,
  day_of_week TINYINT NOT NULL,
  start_time CHAR(5) NOT NULL,
  end_time CHAR(5) NOT NULL,
  start_date DATE NULL,
  end_date DATE NULL,
  status VARCHAR(32) NOT NULL DEFAULT '已确认',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  KEY idx_schedule_teacher_time (teacher_id, day_of_week, start_time, end_time),
  KEY idx_schedule_room_time (campus_id, room_name, day_of_week, start_time, end_time),
  KEY idx_schedule_course (course_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS schedule_class_students (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  schedule_class_id VARCHAR(64) NOT NULL,
  student_id VARCHAR(64) NOT NULL,
  student_name VARCHAR(64) NOT NULL DEFAULT '',
  UNIQUE KEY uk_schedule_student (schedule_class_id, student_id),
  KEY idx_schedule_student_time (student_id, schedule_class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS materials (
  id VARCHAR(64) PRIMARY KEY,
  learning_space_id VARCHAR(64) NOT NULL DEFAULT '',
  course_id VARCHAR(64) NOT NULL,
  title VARCHAR(128) NOT NULL,
  chapter_name VARCHAR(128) NOT NULL,
  material_type VARCHAR(32) NOT NULL,
  owner_teacher_id VARCHAR(64) NOT NULL DEFAULT '',
  owner_teacher_name VARCHAR(64) NOT NULL DEFAULT '',
  publish_status VARCHAR(32) NOT NULL DEFAULT '已发布',
  status VARCHAR(32) NOT NULL DEFAULT '启用',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS homework_tasks (
  id VARCHAR(64) PRIMARY KEY,
  learning_space_id VARCHAR(64) NOT NULL DEFAULT '',
  course_id VARCHAR(64) NOT NULL,
  title VARCHAR(128) NOT NULL,
  grade VARCHAR(32) NOT NULL DEFAULT '',
  semester VARCHAR(32) NOT NULL DEFAULT '',
  subject VARCHAR(32) NOT NULL DEFAULT '',
  question_ids_json TEXT NOT NULL,
  deadline DATE NULL,
  owner_teacher_id VARCHAR(64) NOT NULL DEFAULT '',
  owner_teacher_name VARCHAR(64) NOT NULL DEFAULT '',
  publish_status VARCHAR(32) NOT NULL DEFAULT '已发布',
  status VARCHAR(32) NOT NULL DEFAULT '草稿',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS question_bank_items (
  id VARCHAR(64) PRIMARY KEY,
  title VARCHAR(128) NOT NULL DEFAULT '',
  grade VARCHAR(32) NOT NULL DEFAULT '',
  semester VARCHAR(32) NOT NULL DEFAULT '',
  subject VARCHAR(32) NOT NULL DEFAULT '',
  question_type VARCHAR(32) NOT NULL DEFAULT '',
  stem TEXT NOT NULL,
  options_json TEXT NOT NULL,
  answer VARCHAR(255) NOT NULL DEFAULT '',
  answers_json TEXT NOT NULL,
  score INT NOT NULL DEFAULT 10,
  status VARCHAR(32) NOT NULL DEFAULT '启用',
  owner_teacher_id VARCHAR(64) NOT NULL DEFAULT '',
  owner_teacher_name VARCHAR(64) NOT NULL DEFAULT '',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS homework_submissions (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  homework_id VARCHAR(64) NOT NULL,
  student_id VARCHAR(64) NOT NULL,
  answer_json JSON NULL,
  objective_score INT NOT NULL DEFAULT 0,
  final_score INT NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL DEFAULT '待批改',
  submitted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS review_feedbacks (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  submission_id BIGINT NOT NULL,
  score INT NOT NULL,
  comment TEXT NOT NULL,
  reward VARCHAR(128) DEFAULT '',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS student_score_records (
  id VARCHAR(64) PRIMARY KEY,
  student_id VARCHAR(64) NOT NULL,
  subject VARCHAR(32) NOT NULL,
  exam_type VARCHAR(32) NOT NULL DEFAULT '阶段测评',
  exam_name VARCHAR(64) NOT NULL,
  exam_date DATE NOT NULL,
  score INT NOT NULL,
  full_score INT NOT NULL,
  average_score INT NOT NULL DEFAULT 0,
  teacher_comment TEXT NOT NULL,
  created_by VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  KEY idx_student_score_student (student_id, subject, exam_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS notices (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  external_id VARCHAR(191) NOT NULL DEFAULT '',
  notice_type VARCHAR(32) NOT NULL,
  title VARCHAR(128) NOT NULL,
  target VARCHAR(128) NOT NULL,
  content TEXT NOT NULL,
  channel VARCHAR(32) NOT NULL DEFAULT '站内通知',
  recipient_open_id VARCHAR(128) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT '待发送',
  failure_reason TEXT NOT NULL,
  related_type VARCHAR(32) NOT NULL DEFAULT '',
  related_id VARCHAR(64) NOT NULL DEFAULT '',
  retry_count INT NOT NULL DEFAULT 0,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS material_access_logs (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  student_id VARCHAR(64) NOT NULL,
  material_id VARCHAR(64) NOT NULL,
  action VARCHAR(32) NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS operation_logs (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  operator_id VARCHAR(64) NOT NULL,
  operator_name VARCHAR(64) NOT NULL,
  action VARCHAR(64) NOT NULL,
  target VARCHAR(128) NOT NULL,
  ip VARCHAR(64) NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL,
  detail TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS system_settings (
  setting_key VARCHAR(64) PRIMARY KEY,
  setting_value TEXT NOT NULL,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS commercial_orders (
  id VARCHAR(64) PRIMARY KEY,
  order_no VARCHAR(64) NOT NULL DEFAULT '',
  student_id VARCHAR(64) NOT NULL DEFAULT '',
  student_name VARCHAR(64) NOT NULL DEFAULT '',
  package_id VARCHAR(64) NOT NULL DEFAULT '',
  package_name VARCHAR(255) NOT NULL DEFAULT '',
  amount_cent INT NOT NULL DEFAULT 0,
  paid_amount_cent INT NOT NULL DEFAULT 0,
  refunded_amount_cent INT NOT NULL DEFAULT 0,
  lesson_total INT NOT NULL DEFAULT 0,
  lesson_consumed INT NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL DEFAULT '',
  contract_status VARCHAR(32) NOT NULL DEFAULT '',
  invoice_status VARCHAR(32) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS commercial_payments (
  id VARCHAR(64) PRIMARY KEY,
  order_id VARCHAR(64) NOT NULL DEFAULT '',
  amount_cent INT NOT NULL DEFAULT 0,
  method VARCHAR(64) NOT NULL DEFAULT '',
  transaction_no VARCHAR(128) NOT NULL DEFAULT '',
  paid_at DATETIME NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS commercial_refunds (
  id VARCHAR(64) PRIMARY KEY,
  order_id VARCHAR(64) NOT NULL DEFAULT '',
  amount_cent INT NOT NULL DEFAULT 0,
  reason TEXT NOT NULL,
  refunded_at DATETIME NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS commercial_contracts (
  id VARCHAR(64) PRIMARY KEY,
  order_id VARCHAR(64) NOT NULL DEFAULT '',
  title VARCHAR(255) NOT NULL DEFAULT '',
  signer VARCHAR(64) NOT NULL DEFAULT '',
  signed_at DATETIME NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS commercial_invoices (
  id VARCHAR(64) PRIMARY KEY,
  order_id VARCHAR(64) NOT NULL DEFAULT '',
  title VARCHAR(255) NOT NULL DEFAULT '',
  tax_no VARCHAR(64) NOT NULL DEFAULT '',
  amount_cent INT NOT NULL DEFAULT 0,
  invoice_no VARCHAR(128) NOT NULL DEFAULT '',
  issued_at DATETIME NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS lesson_consumptions (
  id VARCHAR(64) PRIMARY KEY,
  order_id VARCHAR(64) NOT NULL DEFAULT '',
  student_id VARCHAR(64) NOT NULL DEFAULT '',
  schedule_class_id VARCHAR(64) NOT NULL DEFAULT '',
  lesson_count INT NOT NULL DEFAULT 0,
  consumed_at DATETIME NOT NULL,
  remark TEXT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS renewal_reminders (
  id VARCHAR(64) PRIMARY KEY,
  order_id VARCHAR(64) NOT NULL DEFAULT '',
  student_id VARCHAR(64) NOT NULL DEFAULT '',
  reason TEXT NOT NULL,
  due_at VARCHAR(32) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS parent_notices (
  id VARCHAR(64) PRIMARY KEY,
  order_id VARCHAR(64) NOT NULL DEFAULT '',
  student_id VARCHAR(64) NOT NULL DEFAULT '',
  title VARCHAR(255) NOT NULL DEFAULT '',
  content TEXT NOT NULL,
  sent_at DATETIME NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT '',
  notice_id VARCHAR(64) NOT NULL DEFAULT '',
  channel VARCHAR(32) NOT NULL DEFAULT '',
  failure_reason TEXT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO roles (code, name, description) VALUES
  ('student', '学生', '学生端学习权限'),
  ('teacher', '老师', '负责范围内上传内容和批改'),
  ('ops_staff', '运营教务', '套餐开通、内容与数据权限'),
  ('campus_admin', '校区管理员', '校区管理'),
  ('super_admin', '超级管理员', '全部后台权限');

INSERT INTO subjects (id, name, status) VALUES
  ('math', '数学', '启用'),
  ('english', '英文', '启用'),
  ('chinese', '语文', '启用'),
  ('integrated-science', '综合科学', '启用'),
  ('science', '科学', '启用'),
  ('geography', '地理', '启用'),
  ('history', '历史', '启用'),
  ('physics', '物理', '启用'),
  ('chemistry', '化学', '启用')
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  status = VALUES(status);

DELETE FROM subjects WHERE id IN ('politics', 'biology');

-- 每年 7 月 1 日进入下一学年，与后台表单默认值保持一致。
SET @seed_academic_year = CONCAT(
  IF(MONTH(CURDATE()) >= 7, YEAR(CURDATE()), YEAR(CURDATE()) - 1),
  '.',
  IF(MONTH(CURDATE()) >= 7, YEAR(CURDATE()) + 1, YEAR(CURDATE())),
  '学年'
);

INSERT IGNORE INTO system_settings (setting_key, setting_value) VALUES
  ('academicYear', @seed_academic_year),
  ('grades', 'G1-G9'),
  ('semesters', 'S1 / S2'),
  ('watermarkRule', '姓名/昵称 + 手机尾号 + 时间 + 学生ID后缀'),
  ('downloadPolicy', '学生端仅安全预览，不提供下载'),
  ('miniProgramDomainStatus', '待确认'),
  ('officialAccountBindingStatus', '待确认'),
  ('templateMessageStatus', '待确认'),
  ('productionApiDomain', '待配置');
UPDATE system_settings SET setting_value = 'G1-G9' WHERE setting_key = 'grades';

DROP PROCEDURE IF EXISTS seed_starline_demo_data;
DELIMITER //
CREATE PROCEDURE seed_starline_demo_data()
BEGIN
DECLARE EXIT HANDLER FOR SQLEXCEPTION
BEGIN
  ROLLBACK;
  RESIGNAL;
END;
START TRANSACTION;

-- 该过程可重复执行：只清理本过程生成的 demo ID，避免旧年级/学科矩阵残留。
DELETE FROM schedule_class_students
WHERE schedule_class_id IN (
  SELECT id FROM schedule_classes WHERE course_id LIKE 'course-g%'
);
DELETE FROM schedule_classes WHERE course_id LIKE 'course-g%';
DELETE FROM student_learning_space_access WHERE learning_space_id LIKE 'space-g%';
DELETE FROM student_package_grants WHERE package_id LIKE 'pkg-g%';
DELETE FROM package_content_types WHERE package_id LIKE 'pkg-g%';
DELETE FROM package_spaces
WHERE package_id LIKE 'pkg-g%' OR learning_space_id LIKE 'space-g%';
DELETE FROM learning_contents WHERE learning_space_id LIKE 'space-g%';
DELETE FROM material_access_logs WHERE material_id LIKE 'mat-g%';
DELETE FROM review_feedbacks
WHERE submission_id IN (
  SELECT id FROM homework_submissions WHERE homework_id LIKE 'hw-g%'
);
DELETE FROM homework_submissions WHERE homework_id LIKE 'hw-g%';
DELETE FROM question_bank_items WHERE id LIKE 'qb-g%';
DELETE FROM materials WHERE id LIKE 'mat-g%' OR learning_space_id LIKE 'space-g%';
DELETE FROM homework_tasks WHERE id LIKE 'hw-g%' OR learning_space_id LIKE 'space-g%';
DELETE FROM courses WHERE id LIKE 'course-g%' OR learning_space_id LIKE 'space-g%';
DELETE FROM study_packages WHERE id LIKE 'pkg-g%';
DELETE FROM teacher_learning_space_access WHERE learning_space_id LIKE 'space-g%';
DELETE FROM learning_spaces WHERE id LIKE 'space-g%';

DROP TEMPORARY TABLE IF EXISTS seed_grades;
CREATE TEMPORARY TABLE seed_grades (
  grade_no INT NOT NULL,
  grade_name VARCHAR(32) NOT NULL,
  PRIMARY KEY (grade_no)
);
INSERT INTO seed_grades (grade_no, grade_name) VALUES
  (1, '一年级'), (2, '二年级'), (3, '三年级'), (4, '四年级'),
  (5, '五年级'), (6, '六年级'), (7, '七年级'), (8, '八年级'),
  (9, '九年级');

DROP TEMPORARY TABLE IF EXISTS seed_subjects;
CREATE TEMPORARY TABLE seed_subjects (
  subject_code VARCHAR(32) NOT NULL,
  subject_name VARCHAR(32) NOT NULL,
  PRIMARY KEY (subject_code)
);
INSERT INTO seed_subjects (subject_code, subject_name) VALUES
  ('math', '数学'), ('english', '英文'), ('chinese', '语文'),
  ('integrated-science', '综合科学'), ('science', '科学'), ('geography', '地理'),
  ('history', '历史'), ('physics', '物理'), ('chemistry', '化学');

-- 年级与学科的开设关系按业务课程矩阵显式维护：
-- G1-G3：数学、英文、语文、综合科学；
-- G4-G5：数学、英文、语文、科学、地理；
-- G6-G7：数学、英文、语文、科学、历史；
-- G8：数学、英文、语文、科学、地理、物理；
-- G9：数学、英文、语文、科学、历史、物理、化学。
DROP TEMPORARY TABLE IF EXISTS seed_grade_subjects;
CREATE TEMPORARY TABLE seed_grade_subjects (
  grade_no INT NOT NULL,
  subject_code VARCHAR(32) NOT NULL,
  PRIMARY KEY (grade_no, subject_code)
);
INSERT INTO seed_grade_subjects (grade_no, subject_code) VALUES
  (1, 'math'), (1, 'english'), (1, 'chinese'), (1, 'integrated-science'),
  (2, 'math'), (2, 'english'), (2, 'chinese'), (2, 'integrated-science'),
  (3, 'math'), (3, 'english'), (3, 'chinese'), (3, 'integrated-science'),
  (4, 'math'), (4, 'english'), (4, 'chinese'), (4, 'science'), (4, 'geography'),
  (5, 'math'), (5, 'english'), (5, 'chinese'), (5, 'science'), (5, 'geography'),
  (6, 'math'), (6, 'english'), (6, 'chinese'), (6, 'science'), (6, 'history'),
  (7, 'math'), (7, 'english'), (7, 'chinese'), (7, 'science'), (7, 'history'),
  (8, 'math'), (8, 'english'), (8, 'chinese'), (8, 'science'), (8, 'geography'), (8, 'physics'),
  (9, 'math'), (9, 'english'), (9, 'chinese'), (9, 'science'), (9, 'history'), (9, 'physics'), (9, 'chemistry');

DROP TEMPORARY TABLE IF EXISTS seed_semesters;
CREATE TEMPORARY TABLE seed_semesters (
  semester_no INT NOT NULL,
  semester_name VARCHAR(32) NOT NULL,
  PRIMARY KEY (semester_no)
);
INSERT INTO seed_semesters (semester_no, semester_name) VALUES
  (1, 'S1'), (2, 'S2');

DROP TEMPORARY TABLE IF EXISTS seed_phases;
CREATE TEMPORARY TABLE seed_phases (
  phase_code VARCHAR(32) NOT NULL,
  phase_name VARCHAR(32) NOT NULL,
  PRIMARY KEY (phase_code)
);
INSERT INTO seed_phases (phase_code, phase_name) VALUES
  ('q1', 'Q1'), ('q2', 'Q2');

DROP TEMPORARY TABLE IF EXISTS seed_package_types;
CREATE TEMPORARY TABLE seed_package_types (
  package_type VARCHAR(32) NOT NULL,
  package_label VARCHAR(32) NOT NULL,
  PRIMARY KEY (package_type)
);
INSERT INTO seed_package_types (package_type, package_label) VALUES
  ('question', '题'),
  ('question_handout', '题+讲义'),
  ('full', '课程+题+讲义');

INSERT IGNORE INTO roles (code, name, description) VALUES
  ('student', '学生', '学生端学习权限'),
  ('teacher', '老师', '负责范围内上传内容和批改'),
  ('ops_staff', '运营教务', '套餐开通、内容与数据权限'),
  ('campus_admin', '校区管理员', '校区管理'),
  ('super_admin', '超级管理员', '全部后台权限');

INSERT IGNORE INTO subjects (id, name, status)
SELECT subject_code, subject_name, '启用'
FROM seed_subjects;

INSERT IGNORE INTO learning_spaces (id, academic_year, grade, subject, semester, phase, name, status)
SELECT
  CONCAT('space-g', LPAD(g.grade_no, 2, '0'), '-', s.subject_code, '-s', sem.semester_no, '-', p.phase_code),
  @seed_academic_year,
  g.grade_name,
  s.subject_name,
  sem.semester_name,
  p.phase_name,
  CONCAT(g.grade_name, s.subject_name, sem.semester_name, p.phase_name),
  '启用'
FROM seed_grade_subjects gs
JOIN seed_grades g ON g.grade_no = gs.grade_no
JOIN seed_subjects s ON s.subject_code = gs.subject_code
CROSS JOIN seed_semesters sem
CROSS JOIN seed_phases p;

INSERT IGNORE INTO study_packages (id, name, academic_year, grade, semester, subject, phase_scope, package_type, status)
SELECT
  CONCAT('pkg-g', LPAD(g.grade_no, 2, '0'), '-', s.subject_code, '-s', sem.semester_no, '-', pt.package_type),
  CONCAT(@seed_academic_year, ' ', g.grade_name, ' ', sem.semester_name, ' ', s.subject_name, ' ', pt.package_label),
  @seed_academic_year,
  g.grade_name,
  sem.semester_name,
  s.subject_name,
  '全学期',
  pt.package_type,
  '启用'
FROM seed_grade_subjects gs
JOIN seed_grades g ON g.grade_no = gs.grade_no
JOIN seed_subjects s ON s.subject_code = gs.subject_code
CROSS JOIN seed_semesters sem
CROSS JOIN seed_package_types pt;

INSERT IGNORE INTO package_spaces (package_id, learning_space_id)
SELECT
  CONCAT('pkg-g', LPAD(g.grade_no, 2, '0'), '-', s.subject_code, '-s', sem.semester_no, '-', pt.package_type),
  CONCAT('space-g', LPAD(g.grade_no, 2, '0'), '-', s.subject_code, '-s', sem.semester_no, '-', p.phase_code)
FROM seed_grade_subjects gs
JOIN seed_grades g ON g.grade_no = gs.grade_no
JOIN seed_subjects s ON s.subject_code = gs.subject_code
CROSS JOIN seed_semesters sem
CROSS JOIN seed_package_types pt
CROSS JOIN seed_phases p;

INSERT IGNORE INTO package_content_types (package_id, content_type)
SELECT id, 'question'
FROM study_packages
WHERE package_type IN ('question', 'question_handout', 'full')
UNION ALL
SELECT id, 'handout'
FROM study_packages
WHERE package_type IN ('question_handout', 'full')
UNION ALL
SELECT id, 'course'
FROM study_packages
WHERE package_type = 'full';

INSERT IGNORE INTO learning_contents (id, learning_space_id, content_type, source_table, source_id, title, chapter_id, status, sort_order)
SELECT
  CONCAT(content_type, '-', TRIM(LEADING 'space-' FROM id)),
  id,
  content_type,
  source_table,
  CONCAT(source_prefix, '-', TRIM(LEADING 'space-' FROM id)),
  title,
  '基础巩固',
  '启用',
  sort_order
FROM (
  SELECT id, 'course' AS content_type, 'courses' AS source_table, 'course' AS source_prefix, CONCAT(name, '课程') AS title, 1 AS sort_order
  FROM learning_spaces
  UNION ALL
  SELECT id, 'handout', 'materials', 'mat', CONCAT(name, '核心讲义'), 2
  FROM learning_spaces
  UNION ALL
  SELECT id, 'question', 'homework_tasks', 'hw', CONCAT(name, '练习题'), 3
  FROM learning_spaces
) generated_contents;

INSERT IGNORE INTO courses (id, learning_space_id, name, subject, grade, status)
SELECT
  CONCAT('course-g', LPAD(g.grade_no, 2, '0'), '-', s.subject_code, '-s', sem.semester_no, '-', p.phase_code),
  CONCAT('space-g', LPAD(g.grade_no, 2, '0'), '-', s.subject_code, '-s', sem.semester_no, '-', p.phase_code),
  CONCAT(g.grade_name, s.subject_name, sem.semester_name, p.phase_name, '课程'),
  s.subject_name,
  g.grade_name,
  '启用'
FROM seed_grade_subjects gs
JOIN seed_grades g ON g.grade_no = gs.grade_no
JOIN seed_subjects s ON s.subject_code = gs.subject_code
CROSS JOIN seed_semesters sem
CROSS JOIN seed_phases p;

INSERT IGNORE INTO materials (id, learning_space_id, course_id, title, chapter_name, material_type, owner_teacher_id, owner_teacher_name, publish_status, status)
SELECT
  CONCAT('mat-g', LPAD(g.grade_no, 2, '0'), '-', s.subject_code, '-s', sem.semester_no, '-', p.phase_code),
  CONCAT('space-g', LPAD(g.grade_no, 2, '0'), '-', s.subject_code, '-s', sem.semester_no, '-', p.phase_code),
  CONCAT('course-g', LPAD(g.grade_no, 2, '0'), '-', s.subject_code, '-s', sem.semester_no, '-', p.phase_code),
  CONCAT(g.grade_name, s.subject_name, sem.semester_name, p.phase_name, '核心讲义'),
  '基础巩固',
  '讲义',
  CONCAT('teacher-', s.subject_code),
  CONCAT(s.subject_name, '老师'),
  '已发布',
  '启用'
FROM seed_grade_subjects gs
JOIN seed_grades g ON g.grade_no = gs.grade_no
JOIN seed_subjects s ON s.subject_code = gs.subject_code
CROSS JOIN seed_semesters sem
CROSS JOIN seed_phases p;

INSERT IGNORE INTO homework_tasks (id, learning_space_id, course_id, title, deadline, owner_teacher_id, owner_teacher_name, publish_status, status)
SELECT
  CONCAT('hw-g', LPAD(g.grade_no, 2, '0'), '-', s.subject_code, '-s', sem.semester_no, '-', p.phase_code),
  CONCAT('space-g', LPAD(g.grade_no, 2, '0'), '-', s.subject_code, '-s', sem.semester_no, '-', p.phase_code),
  CONCAT('course-g', LPAD(g.grade_no, 2, '0'), '-', s.subject_code, '-s', sem.semester_no, '-', p.phase_code),
  CONCAT(g.grade_name, s.subject_name, sem.semester_name, p.phase_name, '练习题'),
  CASE
    WHEN sem.semester_no = 1 AND p.phase_code = 'q1' THEN '2026-10-30'
    WHEN sem.semester_no = 1 AND p.phase_code = 'q2' THEN '2027-01-15'
    WHEN sem.semester_no = 2 AND p.phase_code = 'q1' THEN '2027-04-30'
    ELSE '2027-06-20'
  END,
  CONCAT('teacher-', s.subject_code),
  CONCAT(s.subject_name, '老师'),
  '已发布',
  '已发布'
FROM seed_grade_subjects gs
JOIN seed_grades g ON g.grade_no = gs.grade_no
JOIN seed_subjects s ON s.subject_code = gs.subject_code
CROSS JOIN seed_semesters sem
CROSS JOIN seed_phases p;

INSERT IGNORE INTO students (id, name, grade, phone, account_status) VALUES
  ('stu-001', '小明', '五年级', '18500009069', '正常'),
  ('stu-002', 'Lucy', '五年级', '13600002201', '待提醒'),
  ('stu-003', '小航', '五年级', '13700003303', '正常');

INSERT IGNORE INTO users (id, name, phone, open_id, account_status, student_id, campus_id, password_hash, must_change_password, token_version) VALUES
  ('user-teacher', '英语老师', '13800000004', 'demo-teacher', '正常', '', 'campus-main', '', 0, 0),
  ('user-student-001', '小明', '18500009069', '', '正常', 'stu-001', 'campus-main', '', 0, 0),
  ('user-student-002', 'Lucy', '13600002201', '', '待提醒', 'stu-002', 'campus-main', '', 0, 0),
  ('user-student-003', '小航', '13700003303', '', '正常', 'stu-003', 'campus-main', '', 0, 0);

INSERT IGNORE INTO user_roles (user_id, role_code) VALUES
  ('user-teacher', 'teacher'),
  ('user-student-001', 'student'),
  ('user-student-002', 'student'),
  ('user-student-003', 'student');

INSERT IGNORE INTO teacher_learning_space_access
  (teacher_id, learning_space_id, can_view, can_upload_handout, can_upload_question, can_review, can_manage_content, status)
VALUES
  ('user-teacher', 'space-g05-english-s1-q1', 1, 1, 1, 1, 1, 'active'),
  ('user-teacher', 'space-g05-english-s1-q2', 1, 1, 1, 1, 1, 'active');

INSERT IGNORE INTO student_package_grants (student_id, package_id, starts_at, ends_at, status, operator_id, operator_name) VALUES
  ('stu-001', 'pkg-g05-english-s1-full', '2026-05-22', '2027-05-22', 'active', 'seed', '初始化'),
  ('stu-002', 'pkg-g05-math-s1-question_handout', '2026-05-22', '2027-05-22', 'active', 'seed', '初始化'),
  ('stu-003', 'pkg-g05-chinese-s1-question', '2026-05-22', '2027-05-22', 'active', 'seed', '初始化'),
  ('stu-002', 'pkg-g05-english-s1-full', '2026-05-22', '2027-05-22', 'active', 'seed', '初始化'),
  ('stu-003', 'pkg-g05-english-s1-question_handout', '2026-05-22', '2027-05-22', 'active', 'seed', '初始化');

INSERT IGNORE INTO student_learning_space_access (student_id, learning_space_id, package_grant_id, starts_at, ends_at, status)
SELECT
  g.student_id,
  ps.learning_space_id,
  g.id,
  g.starts_at,
  g.ends_at,
  g.status
FROM student_package_grants g
JOIN package_spaces ps ON ps.package_id = g.package_id
WHERE g.package_id IN (
  'pkg-g05-english-s1-full',
  'pkg-g05-math-s1-question_handout',
  'pkg-g05-chinese-s1-question',
  'pkg-g05-english-s1-question_handout'
)
  AND g.status = 'active';
COMMIT;
END//
DELIMITER ;
