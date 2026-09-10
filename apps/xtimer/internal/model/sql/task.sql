CREATE TABLE IF NOT EXISTS `task`
(
    `id`         bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    `app`        varchar(255) NOT NULL COMMENT 'app name',
    `timer_id`   bigint(20) NOT NULL COMMENT 'timer id',
    `output`     varchar(256) DEFAULT NULL COMMENT 'execution result',
    `run_timer`  datetime     NOT NULL COMMENT 'execution time',
    `cost_time`  int(8) DEFAULT NULL COMMENT 'execution cost',
    `status`     int(4) NOT NULL COMMENT 'current status',
    `created_at` datetime     NOT NULL COMMENT 'create time',
    `updated_at` datetime     NOT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT 'update time',
    `deleted_at` datetime     DEFAULT NULL COMMENT 'delete time',
    PRIMARY KEY (`id`) USING BTREE COMMENT 'primary key index',
    UNIQUE KEY `idx_def_timer` (`timer_id`,`run_timer`) USING BTREE COMMENT 'timer execution time unique index',
    KEY `idx_run_timer` (`run_timer`) COMMENT 'execution time index'
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4;
