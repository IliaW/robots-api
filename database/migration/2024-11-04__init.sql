CREATE SCHEMA IF NOT EXISTS url_scraper DEFAULT CHARACTER SET utf8 COLLATE utf8_general_ci;
GRANT ALL PRIVILEGES ON url_scraper.* TO 'admin'@'%';
USE url_scraper;

CREATE TABLE IF NOT EXISTS scrape_metadata
(
    id                    INT AUTO_INCREMENT PRIMARY KEY,
    url                   VARCHAR(1000) NOT NULL,
    time_to_scrape        INT           NOT NULL, -- in milliseconds
    timestamp             TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status                VARCHAR(1000) NOT NULL, -- may contain an error message
    status_code           SMALLINT      NOT NULL,
    scrape_mechanism      VARCHAR(30)   NOT NULL,
    scrape_worker_version VARCHAR(30)   NOT NULL,
    e_tag                 VARCHAR(255)  NULL
) ENGINE = InnoDB
  CHARSET = utf8;

CREATE TABLE IF NOT EXISTS custom_rule
(
    id         INT AUTO_INCREMENT PRIMARY KEY,
    domain     VARCHAR(80) NOT NULL UNIQUE,
    robots_txt TEXT        NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX domain_index (domain)
) ENGINE = InnoDB
  CHARSET = utf8;

CREATE TABLE IF NOT EXISTS assessor_api_key
(
    id         INT AUTO_INCREMENT PRIMARY KEY,
    api_key    VARCHAR(64)  NOT NULL UNIQUE,
    email      VARCHAR(100) NOT NULL,
    is_active  BOOLEAN   DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE = InnoDB
  CHARSET = utf8;

DELIMITER $$
CREATE TRIGGER before_insert_assessor_api_key
    BEFORE INSERT
    ON assessor_api_key
    FOR EACH ROW
BEGIN
    SET NEW.api_key = SHA2(NEW.api_key, 256);
END$$

DELIMITER ;
