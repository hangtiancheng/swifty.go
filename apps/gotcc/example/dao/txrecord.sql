CREATE TABLE IF NOT EXISTS `tx_record`
(
    `id`                       bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    `status`                   varchar(16) NOT NULL COMMENT 'transaction status: hanging/successful/failure',
    `component_try_statuses`   json DEFAULT NULL COMMENT 'try-phase status of each component: hanging/successful/failure',
    `deleted_at`        datetime     DEFAULT NULL COMMENT 'deletion time',
    `created_at`        datetime     NOT NULL COMMENT 'creation time',
    `updated_at`        datetime     DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT 'update time',
    PRIMARY KEY (`id`) USING BTREE COMMENT 'primary key index',
    KEY `idx_status` (`status`) COMMENT 'transaction status index'
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4 COMMENT 'transaction log records';
