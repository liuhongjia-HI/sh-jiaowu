ALTER TABLE homework_tasks
  ADD COLUMN allow_download TINYINT(1) NOT NULL DEFAULT 0 AFTER status;
