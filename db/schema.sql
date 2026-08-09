CREATE TABLE IF NOT EXISTS users (
  id CHAR(36) PRIMARY KEY,
  email VARCHAR(320) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  theme_preference ENUM('dark','light') NOT NULL DEFAULT 'dark',
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
	location_lat DECIMAL(9,6) NULL,
	location_lng DECIMAL(9,6) NULL,
	user_type ENUM('PLAYER','DEV','BOT') NOT NULL DEFAULT 'PLAYER',
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
	UNIQUE KEY uq_nations_leader_name (leader_name),
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
DROP TABLE IF EXISTS technology_investments;
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

CREATE TABLE IF NOT EXISTS daily_login_rewards (
  nation_id CHAR(36) NOT NULL,
  reward_date DATE NOT NULL,
  streak INT NOT NULL,
  amount BIGINT NOT NULL DEFAULT 25000,
  awarded_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (nation_id, reward_date),
  CONSTRAINT fk_daily_login_nation FOREIGN KEY (nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  INDEX idx_daily_login_history (nation_id, reward_date)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS city_improvements (
  id CHAR(36) PRIMARY KEY,
  city_id CHAR(36) NOT NULL,
  building_type VARCHAR(50) NOT NULL,
  quantity INT NOT NULL DEFAULT 1,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_city_building (city_id, building_type),
  CONSTRAINT fk_city_improvements_city FOREIGN KEY (city_id) REFERENCES cities(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS national_projects (
  id CHAR(36) PRIMARY KEY,
  nation_id CHAR(36) NOT NULL,
  project_type VARCHAR(50) NOT NULL,
  completed_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_nation_project (nation_id, project_type),
  CONSTRAINT fk_national_projects_nation FOREIGN KEY (nation_id) REFERENCES nations(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS economic_snapshots (
  id CHAR(36) PRIMARY KEY,
  nation_id CHAR(36) NOT NULL,
  turn_at DATETIME NOT NULL,
  cash_income BIGINT NOT NULL,
  upkeep BIGINT NOT NULL,
  population_change BIGINT NOT NULL,
  happiness DECIMAL(6,3) NOT NULL,
  education DECIMAL(6,3) NOT NULL,
  breakdown JSON NOT NULL,
  UNIQUE KEY uq_economic_snapshot (nation_id, turn_at),
  CONSTRAINT fk_snapshots_nation FOREIGN KEY (nation_id) REFERENCES nations(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliances (
  id CHAR(36) PRIMARY KEY,
  founder_nation_id CHAR(36) NOT NULL,
  name VARCHAR(80) NOT NULL UNIQUE,
  description VARCHAR(1000) NOT NULL DEFAULT '',
  emblem_url VARCHAR(500) NOT NULL DEFAULT '',
  community_url VARCHAR(500) NOT NULL DEFAULT '',
  join_policy ENUM('open','apply','invite_only') NOT NULL DEFAULT 'apply',
  minimum_cities INT NOT NULL DEFAULT 1,
  minimum_age_days INT NOT NULL DEFAULT 0,
  minimum_infrastructure INT NOT NULL DEFAULT 0,
  tax_rate DECIMAL(5,2) NOT NULL DEFAULT 0,
  level INT NOT NULL DEFAULT 1,
  experience BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_alliance_founder FOREIGN KEY(founder_nation_id) REFERENCES nations(id),
  CHECK(tax_rate BETWEEN 0 AND 100)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliance_roles (
  id CHAR(36) PRIMARY KEY,
  alliance_id CHAR(36) NOT NULL,
  title VARCHAR(60) NOT NULL,
  rank_order INT NOT NULL,
  can_manage_bank BOOLEAN NOT NULL DEFAULT FALSE,
  can_set_tax BOOLEAN NOT NULL DEFAULT FALSE,
  can_manage_members BOOLEAN NOT NULL DEFAULT FALSE,
  can_declare_war BOOLEAN NOT NULL DEFAULT FALSE,
  can_post_announcements BOOLEAN NOT NULL DEFAULT FALSE,
  daily_withdrawal_limit BIGINT NOT NULL DEFAULT 0,
  UNIQUE KEY uq_alliance_role_title(alliance_id,title),
  CONSTRAINT fk_alliance_roles_alliance FOREIGN KEY(alliance_id) REFERENCES alliances(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliance_members (
  alliance_id CHAR(36) NOT NULL,
  nation_id CHAR(36) NOT NULL,
  role_id CHAR(36) NOT NULL,
  cash_contributed BIGINT NOT NULL DEFAULT 0,
  resources_contributed BIGINT NOT NULL DEFAULT 0,
  joined_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(alliance_id,nation_id),
  UNIQUE KEY uq_nation_alliance(nation_id),
  CONSTRAINT fk_members_alliance FOREIGN KEY(alliance_id) REFERENCES alliances(id) ON DELETE CASCADE,
  CONSTRAINT fk_members_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  CONSTRAINT fk_members_role FOREIGN KEY(role_id) REFERENCES alliance_roles(id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliance_applications (
  id CHAR(36) PRIMARY KEY,
  alliance_id CHAR(36) NOT NULL,
  nation_id CHAR(36) NOT NULL,
  message VARCHAR(500) NOT NULL DEFAULT '',
  status ENUM('pending','accepted','rejected','withdrawn') NOT NULL DEFAULT 'pending',
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  resolved_at TIMESTAMP(6) NULL,
  resolved_by CHAR(36) NULL,
  INDEX idx_alliance_applications(alliance_id,status,created_at),
  CONSTRAINT fk_applications_alliance FOREIGN KEY(alliance_id) REFERENCES alliances(id) ON DELETE CASCADE,
  CONSTRAINT fk_applications_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliance_bank (
  alliance_id CHAR(36) PRIMARY KEY,
  cash BIGINT NOT NULL DEFAULT 0, food BIGINT NOT NULL DEFAULT 0, coal BIGINT NOT NULL DEFAULT 0,
  iron BIGINT NOT NULL DEFAULT 0, oil BIGINT NOT NULL DEFAULT 0, bauxite BIGINT NOT NULL DEFAULT 0,
  steel BIGINT NOT NULL DEFAULT 0, aluminum BIGINT NOT NULL DEFAULT 0, gasoline BIGINT NOT NULL DEFAULT 0,
  munitions BIGINT NOT NULL DEFAULT 0, uranium BIGINT NOT NULL DEFAULT 0,
  CONSTRAINT fk_alliance_bank_alliance FOREIGN KEY(alliance_id) REFERENCES alliances(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliance_bank_transactions (
  id CHAR(36) PRIMARY KEY, alliance_id CHAR(36) NOT NULL, actor_nation_id CHAR(36) NULL,
  recipient_nation_id CHAR(36) NULL, kind ENUM('deposit','withdrawal','grant','tax','loan','repayment') NOT NULL,
  resource VARCHAR(30) NOT NULL, amount BIGINT NOT NULL, memo VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  INDEX idx_alliance_bank_log(alliance_id,created_at),
  CONSTRAINT fk_bank_log_alliance FOREIGN KEY(alliance_id) REFERENCES alliances(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliance_stockpiles (
  alliance_id CHAR(36) NOT NULL,
  commodity VARCHAR(40) NOT NULL,
  amount DECIMAL(20,3) NOT NULL DEFAULT 0,
  PRIMARY KEY(alliance_id,commodity),
  CONSTRAINT fk_alliance_stockpiles_alliance FOREIGN KEY(alliance_id) REFERENCES alliances(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliance_announcements (
  id CHAR(36) PRIMARY KEY, alliance_id CHAR(36) NOT NULL, author_nation_id CHAR(36) NOT NULL,
  title VARCHAR(120) NOT NULL, body TEXT NOT NULL, created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_announcements_alliance FOREIGN KEY(alliance_id) REFERENCES alliances(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliance_loans (
  id CHAR(36) PRIMARY KEY, alliance_id CHAR(36) NOT NULL, nation_id CHAR(36) NOT NULL,
  principal BIGINT NOT NULL, outstanding BIGINT NOT NULL, interest_rate DECIMAL(6,3) NOT NULL DEFAULT 0,
  status ENUM('active','repaid','defaulted','forgiven') NOT NULL DEFAULT 'active', due_at TIMESTAMP(6) NULL,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_loans_alliance FOREIGN KEY(alliance_id) REFERENCES alliances(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliance_treaties (
  id CHAR(36) PRIMARY KEY, alliance_a_id CHAR(36) NOT NULL, alliance_b_id CHAR(36) NOT NULL,
  treaty_type ENUM('NAP','MDP','MDoAP','protectorate','trade') NOT NULL, status ENUM('proposed','active','cancelled') NOT NULL DEFAULT 'proposed',
  terms TEXT NOT NULL, created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS nation_economic_strategy (
  nation_id CHAR(36) PRIMARY KEY,
  gear ENUM('balanced','agrarian','industrial','commercial','militarized') NOT NULL DEFAULT 'balanced',
  political_capital DECIMAL(10,3) NOT NULL DEFAULT 100,
  gear_changed_at TIMESTAMP(6) NULL,
  disruption_until TIMESTAMP(6) NULL,
  CONSTRAINT fk_strategy_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS province_economies (
  city_id CHAR(36) PRIMARY KEY,
  specialization ENUM('mixed','agriculture','extraction','industry','commerce','military') NOT NULL DEFAULT 'mixed',
  development_level INT NOT NULL DEFAULT 1,
  latitude DECIMAL(9,6) NOT NULL DEFAULT 0,
  longitude DECIMAL(9,6) NOT NULL DEFAULT 0,
  CONSTRAINT fk_province_economy_city FOREIGN KEY(city_id) REFERENCES cities(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS province_upgrades (
  city_id CHAR(36) NOT NULL,
  upgrade_key ENUM('agriculture','extraction','light_industry','heavy_industry','commerce','civil','military_industry') NOT NULL,
  level INT NOT NULL DEFAULT 0,
  total_invested BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY(city_id,upgrade_key),
  CONSTRAINT fk_province_upgrades_city FOREIGN KEY(city_id) REFERENCES cities(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS province_deposits (
  city_id CHAR(36) NOT NULL,
  resource ENUM('foodstuffs','timber','fibers','basic_metals','energy','strategic_minerals') NOT NULL,
  richness DECIMAL(6,3) NOT NULL,
  PRIMARY KEY(city_id,resource),
  CONSTRAINT fk_deposits_city FOREIGN KEY(city_id) REFERENCES cities(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS nation_stockpiles (
  nation_id CHAR(36) NOT NULL,
  commodity VARCHAR(40) NOT NULL,
  amount DECIMAL(20,3) NOT NULL DEFAULT 0,
  PRIMARY KEY(nation_id,commodity),
  CONSTRAINT fk_stockpiles_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS social_policy_selections (
  nation_id CHAR(36) NOT NULL,
  policy_key VARCHAR(50) NOT NULL,
  activated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(nation_id,policy_key),
  CONSTRAINT fk_social_policy_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS production_quotas (
  nation_id CHAR(36) NOT NULL,
  commodity VARCHAR(40) NOT NULL,
  priority DECIMAL(5,2) NOT NULL DEFAULT 0,
  PRIMARY KEY(nation_id,commodity),
  CONSTRAINT fk_quotas_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS trade_shipments (
  id CHAR(36) PRIMARY KEY,
  seller_nation_id CHAR(36) NOT NULL,
  buyer_nation_id CHAR(36) NOT NULL,
  commodity VARCHAR(40) NOT NULL,
  quantity DECIMAL(20,3) NOT NULL,
  unit_price BIGINT NOT NULL,
  distance_km DECIMAL(12,3) NOT NULL,
  shipping_cost BIGINT NOT NULL,
  status ENUM('preparing','in_transit','delivered','disrupted','cancelled') NOT NULL DEFAULT 'preparing',
  departs_at TIMESTAMP(6) NULL,
  arrives_at TIMESTAMP(6) NULL,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  INDEX idx_shipments_arrival(status,arrives_at),
  CONSTRAINT fk_shipments_seller FOREIGN KEY(seller_nation_id) REFERENCES nations(id),
  CONSTRAINT fk_shipments_buyer FOREIGN KEY(buyer_nation_id) REFERENCES nations(id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS notifications (
  id CHAR(36) PRIMARY KEY,
  nation_id CHAR(36) NOT NULL,
  category ENUM('economic','war','market','game') NOT NULL,
  title VARCHAR(120) NOT NULL,
  message VARCHAR(1000) NOT NULL,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  read_at TIMESTAMP(6) NULL,
  INDEX idx_notifications_feed(nation_id,created_at),
  INDEX idx_notifications_unread(nation_id,read_at),
  CONSTRAINT fk_notifications_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE
) ENGINE=InnoDB;
