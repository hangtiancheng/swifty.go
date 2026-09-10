-- Schema for xtimer, matching the models in internal/model/po.
-- Apply with: mysql -u<user> -p <database> < internal/model/sql/xtimer.sql

CREATE TABLE IF NOT EXISTS `timer` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(3) NULL DEFAULT NULL,
  `updated_at` DATETIME(3) NULL DEFAULT NULL,
  `deleted_at` DATETIME(3) NULL DEFAULT NULL,
  `app` VARCHAR(255) NOT NULL COMMENT 'app the timer belongs to',
  `name` VARCHAR(255) NOT NULL COMMENT 'timer definition name',
  `status` INT NOT NULL COMMENT 'timer definition status, 1: disabled, 2: enabled',
  `cron` VARCHAR(255) NOT NULL COMMENT 'timer cron configuration',
  `notify_http_param` TEXT NOT NULL COMMENT 'HTTP callback parameters',
  PRIMARY KEY (`id`),
  KEY `idx_timer_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `task` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(3) NULL DEFAULT NULL,
  `updated_at` DATETIME(3) NULL DEFAULT NULL,
  `deleted_at` DATETIME(3) NULL DEFAULT NULL,
  `app` VARCHAR(255) NOT NULL COMMENT 'app the timer belongs to',
  `timer_id` BIGINT UNSIGNED NOT NULL COMMENT 'timer definition ID',
  `output` TEXT NULL COMMENT 'execution result',
  `run_timer` DATETIME(3) NULL DEFAULT NULL COMMENT 'execution time',
  `cost_time` INT NOT NULL DEFAULT 0 COMMENT 'execution cost in milliseconds',
  `status` INT NOT NULL COMMENT 'current status, 0: not run, 1: running, 2: succeeded, 3: failed',
  PRIMARY KEY (`id`),
  -- Prevents duplicate task inserts when a timer is enabled repeatedly.
  UNIQUE KEY `uk_task_timer_id_run_timer` (`timer_id`, `run_timer`),
  KEY `idx_task_deleted_at` (`deleted_at`),
  KEY `idx_task_run_timer` (`run_timer`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
