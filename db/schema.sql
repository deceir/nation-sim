CREATE TABLE IF NOT EXISTS users (
  id CHAR(36) PRIMARY KEY,
  email VARCHAR(320) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS sessions (
  token_hash CHAR(64) PRIMARY KEY,
  user_id CHAR(36) NOT NULL,
  expires_at TIMESTAMP(6) NOT NULL,
  last_action_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  INDEX idx_sessions_expiry (expires_at)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS nations (
  id CHAR(36) PRIMARY KEY,
  owner_id CHAR(36) NOT NULL UNIQUE,
  name VARCHAR(100) NOT NULL UNIQUE,
	leader_name VARCHAR(100) NOT NULL,
	government_type VARCHAR(60) NOT NULL,
	continent VARCHAR(30) NOT NULL,
  motto VARCHAR(120) NOT NULL DEFAULT '',
  currency_name VARCHAR(30) NOT NULL DEFAULT 'Yen',
  treasury BIGINT NOT NULL DEFAULT 100000,
  coal BIGINT NOT NULL DEFAULT 500,
  steel BIGINT NOT NULL DEFAULT 250,
  food BIGINT NOT NULL DEFAULT 1000,
  population BIGINT NOT NULL DEFAULT 100000,
  employment_rate DECIMAL(5,2) NOT NULL DEFAULT 72.00,
  happiness INT NOT NULL DEFAULT 65 CHECK (happiness BETWEEN 0 AND 100),
  education INT NOT NULL DEFAULT 40 CHECK (education BETWEEN 0 AND 100),
  technology INT NOT NULL DEFAULT 20 CHECK (technology BETWEEN 0 AND 100),
  quality_of_life INT NOT NULL DEFAULT 45 CHECK (quality_of_life BETWEEN 0 AND 100),
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_nations_owner FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS cities (
  id CHAR(36) PRIMARY KEY,
  nation_id CHAR(36) NOT NULL,
  name VARCHAR(100) NOT NULL,
  level INT NOT NULL DEFAULT 1,
  infrastructure INT NOT NULL DEFAULT 100,
  total_invested BIGINT NOT NULL DEFAULT 0,
  improvement_slots INT NOT NULL DEFAULT 2,
  population_capacity BIGINT NOT NULL DEFAULT 100000,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_city_name (nation_id, name),
  CONSTRAINT fk_cities_nation FOREIGN KEY (nation_id) REFERENCES nations(id) ON DELETE CASCADE
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS city_investments (id CHAR(36) PRIMARY KEY,city_id CHAR(36) NOT NULL,nation_id CHAR(36) NOT NULL,program VARCHAR(40) NOT NULL,amount BIGINT NOT NULL,created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),CONSTRAINT fk_city_investment_city FOREIGN KEY(city_id) REFERENCES cities(id) ON DELETE CASCADE,CONSTRAINT fk_city_investment_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE,INDEX idx_city_investments_daily(nation_id,created_at)) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS city_industries (id CHAR(36) PRIMARY KEY,city_id CHAR(36) NOT NULL,resource ENUM('coal','steel','food') NOT NULL,level INT NOT NULL DEFAULT 1,total_invested BIGINT NOT NULL,UNIQUE KEY uq_city_industry(city_id,resource),CONSTRAINT fk_industries_city FOREIGN KEY(city_id) REFERENCES cities(id) ON DELETE CASCADE) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS technology_investments (id CHAR(36) PRIMARY KEY,nation_id CHAR(36) NOT NULL,program VARCHAR(50) NOT NULL,amount BIGINT NOT NULL,created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),CONSTRAINT fk_technology_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE,INDEX idx_technology_limits(nation_id,program,created_at)) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS economy_turns (turn_at DATETIME PRIMARY KEY,processed_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),nations_processed INT NOT NULL DEFAULT 0) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS guardian_grants (
  id CHAR(36) PRIMARY KEY,
  nation_id CHAR(36) NOT NULL,
  starts_at TIMESTAMP(6) NOT NULL,
  expires_at TIMESTAMP(6) NOT NULL,
  reason VARCHAR(100) NOT NULL,
  granted_by VARCHAR(100) NOT NULL,
  revoked_at TIMESTAMP(6) NULL,
  revoked_reason VARCHAR(100) NULL,
  CHECK (expires_at > starts_at),
  CONSTRAINT fk_guardian_nation FOREIGN KEY (nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  INDEX idx_guardian_lookup (nation_id, revoked_at, starts_at, expires_at)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS market_orders (
  id CHAR(36) PRIMARY KEY,
  nation_id CHAR(36) NOT NULL,
  side ENUM('buy','sell') NOT NULL,
  resource ENUM('coal','steel','food') NOT NULL,
  quantity BIGINT NOT NULL CHECK (quantity > 0),
  remaining BIGINT NOT NULL CHECK (remaining >= 0),
  unit_price BIGINT NOT NULL CHECK (unit_price > 0),
  status ENUM('open','filled','cancelled') NOT NULL DEFAULT 'open',
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_orders_nation FOREIGN KEY (nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  INDEX idx_orders_book (resource, side, status, unit_price, created_at)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS conflicts (
  id CHAR(36) PRIMARY KEY,
  kind ENUM('raid','war') NOT NULL,
  attacker_id CHAR(36) NOT NULL,
  defender_id CHAR(36) NOT NULL,
  status VARCHAR(30) NOT NULL DEFAULT 'declared',
  declared_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CHECK (attacker_id <> defender_id),
  CONSTRAINT fk_conflicts_attacker FOREIGN KEY (attacker_id) REFERENCES nations(id),
  CONSTRAINT fk_conflicts_defender FOREIGN KEY (defender_id) REFERENCES nations(id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS ledger_entries (
  id CHAR(36) PRIMARY KEY,
  nation_id CHAR(36) NOT NULL,
  category VARCHAR(50) NOT NULL,
  amount BIGINT NOT NULL,
  memo VARCHAR(255) NOT NULL,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_ledger_nation FOREIGN KEY (nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  INDEX idx_ledger_nation_time (nation_id, created_at)
) ENGINE=InnoDB;
