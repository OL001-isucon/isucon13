CREATE TABLE `comments` (
  `id` int NOT NULL AUTO_INCREMENT,
  `domain_id` int NOT NULL,
  `name` varchar(255) NOT NULL,
  `type` varchar(10) NOT NULL,
  `modified_at` int NOT NULL,
  `account` varchar(40) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci DEFAULT NULL,
  `comment` text CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  PRIMARY KEY (`id`),
  KEY `comments_name_type_idx` (`name`,`type`),
  KEY `comments_order_idx` (`domain_id`,`modified_at`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

CREATE TABLE `cryptokeys` (
  `id` int NOT NULL AUTO_INCREMENT,
  `domain_id` int NOT NULL,
  `flags` int NOT NULL,
  `active` tinyint(1) DEFAULT NULL,
  `published` tinyint(1) DEFAULT '1',
  `content` text,
  PRIMARY KEY (`id`),
  KEY `domainidindex` (`domain_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

CREATE TABLE `domainmetadata` (
  `id` int NOT NULL AUTO_INCREMENT,
  `domain_id` int NOT NULL,
  `kind` varchar(32) DEFAULT NULL,
  `content` text,
  PRIMARY KEY (`id`),
  KEY `domainmetadata_idx` (`domain_id`,`kind`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

CREATE TABLE `domains` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `master` varchar(128) DEFAULT NULL,
  `last_check` int DEFAULT NULL,
  `type` varchar(8) NOT NULL,
  `notified_serial` int unsigned DEFAULT NULL,
  `account` varchar(40) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci DEFAULT NULL,
  `options` varchar(64000) DEFAULT NULL,
  `catalog` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `name_index` (`name`),
  KEY `catalog_idx` (`catalog`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=latin1;

CREATE TABLE `records` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `domain_id` int DEFAULT NULL,
  `name` varchar(255) DEFAULT NULL,
  `type` varchar(10) DEFAULT NULL,
  `content` varchar(64000) DEFAULT NULL,
  `ttl` int DEFAULT NULL,
  `prio` int DEFAULT NULL,
  `disabled` tinyint(1) DEFAULT '0',
  `ordername` varchar(255) CHARACTER SET latin1 COLLATE latin1_bin DEFAULT NULL,
  `auth` tinyint(1) DEFAULT '1',
  PRIMARY KEY (`id`),
  KEY `domain_id` (`domain_id`),
  KEY `did_name_dld` (`domain_id`,`name`,`disabled`),
  KEY `ordername` (`ordername`)
) ENGINE=InnoDB AUTO_INCREMENT=76341 DEFAULT CHARSET=latin1;

CREATE TABLE `supermasters` (
  `ip` varchar(64) NOT NULL,
  `nameserver` varchar(255) NOT NULL,
  `account` varchar(40) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  PRIMARY KEY (`ip`,`nameserver`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

CREATE TABLE `tsigkeys` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `algorithm` varchar(50) DEFAULT NULL,
  `secret` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `namealgoindex` (`name`,`algorithm`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
