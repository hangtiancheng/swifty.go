CREATE TABLE IF NOT EXISTS `timer`
(
    `id`                bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    `app`               varchar(255) NOT NULL COMMENT 'app name',
    `name`              varchar(255) NOT NULL COMMENT 'timer name',
    `status`            smallint(255) NOT NULL COMMENT 'timer status, 1 disabled, 2 enabled',
    `cron`              varchar(255) NOT NULL COMMENT 'cron expression',
    `notify_http_param` json         DEFAULT NULL COMMENT 'http callback parameters',
    `deleted_at`        datetime     DEFAULT NULL COMMENT 'delete time',
    `created_at`        datetime     NOT NULL COMMENT 'create time',
    `updated_at`        datetime     DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT 'update time',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uni_app` (`app`,`name`) USING BTREE COMMENT 'app and name index'
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8;
