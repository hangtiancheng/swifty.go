CREATE TABLE IF NOT EXISTS `example`
(
    `id`                       bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key id',
    `key`                      varchar(64) NOT NULL COMMENT 'unique data key',
    `data`                     varchar(64) NOT NULL COMMENT 'data content',
    PRIMARY KEY (`id`) USING BTREE COMMENT 'primary key index',
    UNIQUE KEY `uniq_key` (`key`)
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4 COMMENT 'consistent cache example table';
