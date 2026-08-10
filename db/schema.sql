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
  capital_city_id CHAR(36) NULL,
  motto VARCHAR(120) NOT NULL DEFAULT '',
  currency_name VARCHAR(30) NOT NULL DEFAULT 'Yen',
  treasury BIGINT NOT NULL DEFAULT 10000000,
  gdp BIGINT NOT NULL DEFAULT 0,
  population BIGINT NOT NULL DEFAULT 100000,
  employment_rate DECIMAL(5,2) NOT NULL DEFAULT 72.00,
  happiness INT NOT NULL DEFAULT 65 CHECK (happiness BETWEEN 0 AND 100),
  education INT NOT NULL DEFAULT 40 CHECK (education BETWEEN 0 AND 100),
  technology INT NOT NULL DEFAULT 20 CHECK (technology BETWEEN 0 AND 100),
  technology_progress DECIMAL(8,4) NOT NULL DEFAULT 0,
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
DROP TABLE IF EXISTS technology_investments;
CREATE TABLE IF NOT EXISTS economy_turns (turn_at DATETIME PRIMARY KEY,processed_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),nations_processed INT NOT NULL DEFAULT 0) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS balance_migrations (migration_key VARCHAR(80) PRIMARY KEY,applied_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB;
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
  resource VARCHAR(40) NOT NULL,
  quantity DECIMAL(20,3) NOT NULL CHECK (quantity > 0),
  remaining DECIMAL(20,3) NOT NULL CHECK (remaining >= 0),
  unit_price BIGINT NOT NULL CHECK (unit_price > 0),
  channel ENUM('public','private') NOT NULL DEFAULT 'public',
  target_nation_id CHAR(36) NULL,
  escrow_cash BIGINT NOT NULL DEFAULT 0,
  escrow_goods DECIMAL(20,3) NOT NULL DEFAULT 0,
  status ENUM('open','pending','filled','cancelled','rejected') NOT NULL DEFAULT 'open',
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_orders_nation FOREIGN KEY (nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  CONSTRAINT fk_orders_target FOREIGN KEY (target_nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  INDEX idx_orders_book (resource, side, status, unit_price, created_at)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS trade_shipments (
  id CHAR(36) PRIMARY KEY,
  order_id CHAR(36) NULL,
  seller_nation_id CHAR(36) NOT NULL,
  buyer_nation_id CHAR(36) NOT NULL,
  resource VARCHAR(40) NOT NULL,
  quantity DECIMAL(20,3) NOT NULL,
  unit_price BIGINT NOT NULL,
  goods_value BIGINT NOT NULL,
  shipping_fee BIGINT NOT NULL,
  distance_modifier DECIMAL(6,3) NOT NULL,
  risk_percent DECIMAL(6,3) NOT NULL,
  turns_total INT NOT NULL,
  turns_remaining INT NOT NULL,
  delay_count INT NOT NULL DEFAULT 0,
  origin_lat DECIMAL(9,6) NOT NULL,
  origin_lng DECIMAL(9,6) NOT NULL,
  destination_lat DECIMAL(9,6) NOT NULL,
  destination_lng DECIMAL(9,6) NOT NULL,
  departed_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  estimated_arrival_at TIMESTAMP(6) NOT NULL,
  delivered_at TIMESTAMP(6) NULL,
  status ENUM('in_transit','delivered','delayed','cancelled') NOT NULL DEFAULT 'in_transit',
  INDEX idx_shipments_due(status,turns_remaining),
  INDEX idx_shipments_parties(seller_nation_id,buyer_nation_id),
  CONSTRAINT fk_shipments_order FOREIGN KEY(order_id) REFERENCES market_orders(id) ON DELETE SET NULL,
  CONSTRAINT fk_shipments_seller FOREIGN KEY(seller_nation_id) REFERENCES nations(id),
  CONSTRAINT fk_shipments_buyer FOREIGN KEY(buyer_nation_id) REFERENCES nations(id)
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
CREATE TABLE IF NOT EXISTS military_inventory (
  nation_id CHAR(36) NOT NULL,
  unit_type ENUM('soldiers','tanks','ships','jets','drones') NOT NULL,
  quantity BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (nation_id,unit_type),
  CONSTRAINT fk_military_inventory_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  CHECK (quantity >= 0)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS military_production_daily (
  nation_id CHAR(36) NOT NULL,
  unit_type ENUM('soldiers','tanks','ships','jets','drones') NOT NULL,
  production_date DATE NOT NULL,
  quantity BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (nation_id,unit_type,production_date),
  CONSTRAINT fk_military_production_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS military_upkeep_state (
  nation_id CHAR(36) PRIMARY KEY,
  cash_fraction DECIMAL(20,6) NOT NULL DEFAULT 0,
  updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_military_upkeep_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE
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

CREATE TABLE IF NOT EXISTS venture_accounts (
  nation_id CHAR(36) PRIMARY KEY,
  personal_capital BIGINT NOT NULL DEFAULT 0,
  transfer_used_today BIGINT NOT NULL DEFAULT 0,
  transfer_date DATE NULL,
  board_refresh_at TIMESTAMP(6) NULL,
  updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_venture_account_nation FOREIGN KEY (nation_id) REFERENCES nations(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS venture_opportunities (
  id CHAR(36) PRIMARY KEY,
  nation_id CHAR(36) NOT NULL,
  template_key VARCHAR(80) NOT NULL,
  title VARCHAR(140) NOT NULL,
  description VARCHAR(600) NOT NULL,
  min_investment BIGINT NOT NULL,
  max_investment BIGINT NOT NULL,
  duration_hours INT NOT NULL,
  risk ENUM('low','medium','high') NOT NULL,
  min_return_bps INT NOT NULL,
  max_return_bps INT NOT NULL,
  expires_at TIMESTAMP(6) NOT NULL,
  accepted_at TIMESTAMP(6) NULL,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  INDEX idx_venture_opportunity_board (nation_id,accepted_at,expires_at),
  CONSTRAINT fk_venture_opportunity_nation FOREIGN KEY (nation_id) REFERENCES nations(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS personal_ventures (
  id CHAR(36) PRIMARY KEY,
  nation_id CHAR(36) NOT NULL,
  opportunity_id CHAR(36) NULL,
  title VARCHAR(140) NOT NULL,
  description VARCHAR(600) NOT NULL,
  risk ENUM('low','medium','high') NOT NULL,
  amount_invested BIGINT NOT NULL,
  outcome_bps INT NULL,
  payout BIGINT NULL,
  status ENUM('active','claimable','collected','cancelled') NOT NULL DEFAULT 'active',
  matures_at TIMESTAMP(6) NOT NULL,
  resolved_at TIMESTAMP(6) NULL,
  collected_at TIMESTAMP(6) NULL,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  INDEX idx_personal_ventures_nation (nation_id,status,created_at),
  INDEX idx_personal_ventures_maturity (status,matures_at),
  CONSTRAINT fk_personal_venture_nation FOREIGN KEY (nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  CONSTRAINT fk_personal_venture_opportunity FOREIGN KEY (opportunity_id) REFERENCES venture_opportunities(id) ON DELETE SET NULL
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS daily_login_rewards (
  nation_id CHAR(36) NOT NULL,
  reward_date DATE NOT NULL,
  streak INT NOT NULL,
  amount BIGINT NOT NULL DEFAULT 2500000,
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

CREATE TABLE IF NOT EXISTS national_long_term_projects (
  id CHAR(36) PRIMARY KEY,
  nation_id CHAR(36) NOT NULL,
  project_type VARCHAR(80) NOT NULL,
  completed_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_long_term_project (nation_id,project_type),
  CONSTRAINT fk_long_term_project_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS national_project_construction (
  id CHAR(36) PRIMARY KEY,
  nation_id CHAR(36) NOT NULL,
  project_type VARCHAR(80) NOT NULL,
  turns_total INT NOT NULL,
  turns_remaining INT NOT NULL,
  cash_locked BIGINT NOT NULL,
  commodities_locked JSON NOT NULL,
  status ENUM('building','complete') NOT NULL DEFAULT 'building',
  started_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  completed_at TIMESTAMP(6) NULL,
  INDEX idx_project_construction_turn(status,turns_remaining),
  CONSTRAINT fk_project_construction_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE
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
  default_key ENUM('leader','member','applicant') NULL,
  can_view_bank BOOLEAN NOT NULL DEFAULT FALSE,
  can_deposit_bank BOOLEAN NOT NULL DEFAULT TRUE,
  can_withdraw_bank BOOLEAN NOT NULL DEFAULT FALSE,
  can_accept_applicants BOOLEAN NOT NULL DEFAULT FALSE,
  can_remove_members BOOLEAN NOT NULL DEFAULT FALSE,
  can_edit_details BOOLEAN NOT NULL DEFAULT FALSE,
  can_manage_roles BOOLEAN NOT NULL DEFAULT FALSE,
  can_promote_members BOOLEAN NOT NULL DEFAULT FALSE,
  can_view_audit_log BOOLEAN NOT NULL DEFAULT FALSE,
  UNIQUE KEY uq_alliance_role_title(alliance_id,title),
  CONSTRAINT fk_alliance_roles_alliance FOREIGN KEY(alliance_id) REFERENCES alliances(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliance_tax_brackets (
  id CHAR(36) PRIMARY KEY,
  alliance_id CHAR(36) NOT NULL,
  name VARCHAR(80) NOT NULL,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  role_id CHAR(36) NULL,
  nation_id CHAR(36) NULL,
  minimum_provinces INT NOT NULL DEFAULT 0,
  cash_rate DECIMAL(5,2) NOT NULL DEFAULT 0,
  resource_rate DECIMAL(5,2) NOT NULL DEFAULT 0,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_alliance_tax_name(alliance_id,name),
  UNIQUE KEY uq_alliance_tax_nation(nation_id),
  CONSTRAINT fk_tax_bracket_alliance FOREIGN KEY(alliance_id) REFERENCES alliances(id) ON DELETE CASCADE,
  CONSTRAINT fk_tax_bracket_role FOREIGN KEY(role_id) REFERENCES alliance_roles(id) ON DELETE SET NULL,
  CONSTRAINT fk_tax_bracket_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  CHECK(cash_rate BETWEEN 0 AND 100),
  CHECK(resource_rate BETWEEN 0 AND 100)
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
  cash BIGINT NOT NULL DEFAULT 0,
  CONSTRAINT fk_alliance_bank_alliance FOREIGN KEY(alliance_id) REFERENCES alliances(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliance_bank_transactions (
  id CHAR(36) PRIMARY KEY, alliance_id CHAR(36) NOT NULL, actor_nation_id CHAR(36) NULL,
  recipient_nation_id CHAR(36) NULL, kind ENUM('deposit','withdrawal','grant','tax','loan','repayment','balance_adjustment') NOT NULL,
  resource VARCHAR(30) NOT NULL, amount DECIMAL(20,3) NOT NULL, memo VARCHAR(255) NOT NULL DEFAULT '', batch_id CHAR(36) NULL,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  INDEX idx_alliance_bank_log(alliance_id,created_at),
  INDEX idx_alliance_bank_batch(alliance_id,batch_id),
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
  proposed_by_alliance_id CHAR(36) NOT NULL, proposed_by_nation_id CHAR(36) NOT NULL,
  treaty_type VARCHAR(16) NOT NULL, status ENUM('proposed','active','rejected','cancelled','expired') NOT NULL DEFAULT 'proposed',
  terms TEXT NOT NULL, duration_days INT NULL, starts_on DATE NULL, ends_on DATE NULL,
  resolved_by_nation_id CHAR(36) NULL, resolved_at TIMESTAMP(6) NULL,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  INDEX idx_treaty_parties(alliance_a_id,alliance_b_id,status),
  CONSTRAINT fk_treaty_a FOREIGN KEY(alliance_a_id) REFERENCES alliances(id) ON DELETE CASCADE,
  CONSTRAINT fk_treaty_b FOREIGN KEY(alliance_b_id) REFERENCES alliances(id) ON DELETE CASCADE,
  CONSTRAINT fk_treaty_proposer_alliance FOREIGN KEY(proposed_by_alliance_id) REFERENCES alliances(id) ON DELETE CASCADE,
  CONSTRAINT fk_treaty_proposer_nation FOREIGN KEY(proposed_by_nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  CONSTRAINT fk_treaty_resolver FOREIGN KEY(resolved_by_nation_id) REFERENCES nations(id) ON DELETE SET NULL
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliance_tax_assignments (
  alliance_id CHAR(36) NOT NULL,
  nation_id CHAR(36) NOT NULL,
  bracket_id CHAR(36) NOT NULL,
  assigned_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(alliance_id,nation_id),
  INDEX idx_tax_assignment_bracket(bracket_id),
  CONSTRAINT fk_tax_assignment_alliance FOREIGN KEY(alliance_id) REFERENCES alliances(id) ON DELETE CASCADE,
  CONSTRAINT fk_tax_assignment_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  CONSTRAINT fk_tax_assignment_bracket FOREIGN KEY(bracket_id) REFERENCES alliance_tax_brackets(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS alliance_member_balances (
  alliance_id CHAR(36) NOT NULL,
  nation_id CHAR(36) NOT NULL,
  resource VARCHAR(40) NOT NULL,
  amount DECIMAL(20,3) NOT NULL DEFAULT 0,
  PRIMARY KEY(alliance_id,nation_id,resource),
  INDEX idx_alliance_member_balances_nation(alliance_id,nation_id),
  CONSTRAINT fk_member_balances_alliance FOREIGN KEY(alliance_id) REFERENCES alliances(id) ON DELETE CASCADE,
  CONSTRAINT fk_member_balances_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE
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

CREATE TABLE IF NOT EXISTS luxury_consumption_settings (
  nation_id CHAR(36) PRIMARY KEY,
  daily_rate DECIMAL(20,3) NOT NULL DEFAULT 0,
  updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_luxury_consumption_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  CHECK(daily_rate >= 0)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS luxury_consumption_history (
  nation_id CHAR(36) NOT NULL,
  server_date DATE NOT NULL,
  requested_rate DECIMAL(20,3) NOT NULL,
  actual_consumed DECIMAL(20,3) NOT NULL,
  size_efficiency DECIMAL(8,4) NOT NULL,
  income_earned BIGINT NOT NULL,
  settled_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(nation_id,server_date),
  CONSTRAINT fk_luxury_history_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  INDEX idx_luxury_history_date(server_date)
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

CREATE TABLE IF NOT EXISTS crisis_templates (
  id VARCHAR(80) PRIMARY KEY,
  internal_name VARCHAR(100) NOT NULL UNIQUE,
  title VARCHAR(160) NOT NULL,
  briefing VARCHAR(1200) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS crisis_options (
  id VARCHAR(100) PRIMARY KEY,
  template_id VARCHAR(80) NOT NULL,
  sort_order INT NOT NULL,
  label VARCHAR(120) NOT NULL,
  description VARCHAR(500) NOT NULL,
  effect_type VARCHAR(50) NOT NULL,
  effect_target VARCHAR(50) NOT NULL DEFAULT '',
  effect_value DECIMAL(14,3) NOT NULL DEFAULT 0,
  effect_text VARCHAR(240) NOT NULL,
  effect_payload JSON NULL,
  UNIQUE KEY uq_crisis_option_order(template_id,sort_order),
  CONSTRAINT fk_crisis_option_template FOREIGN KEY(template_id) REFERENCES crisis_templates(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS crisis_days (
  server_date DATE PRIMARY KEY,
  crisis_count INT NOT NULL,
  generated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS daily_crises (
  id CHAR(36) PRIMARY KEY,
  server_date DATE NOT NULL,
  template_id VARCHAR(80) NOT NULL,
  slot_number INT NOT NULL,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_daily_crisis_template(server_date,template_id),
  UNIQUE KEY uq_daily_crisis_slot(server_date,slot_number),
  INDEX idx_daily_crisis_date(server_date),
  CONSTRAINT fk_daily_crisis_day FOREIGN KEY(server_date) REFERENCES crisis_days(server_date) ON DELETE CASCADE,
  CONSTRAINT fk_daily_crisis_template FOREIGN KEY(template_id) REFERENCES crisis_templates(id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS nation_crisis_responses (
  nation_id CHAR(36) NOT NULL,
  daily_crisis_id CHAR(36) NOT NULL,
  option_id VARCHAR(100) NOT NULL,
  effect_summary VARCHAR(240) NOT NULL,
  responded_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(nation_id,daily_crisis_id),
  INDEX idx_crisis_response_nation_time(nation_id,responded_at),
  CONSTRAINT fk_crisis_response_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  CONSTRAINT fk_crisis_response_daily FOREIGN KEY(daily_crisis_id) REFERENCES daily_crises(id) ON DELETE CASCADE,
  CONSTRAINT fk_crisis_response_option FOREIGN KEY(option_id) REFERENCES crisis_options(id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS crisis_modifiers (
  nation_id CHAR(36) NOT NULL,
  daily_crisis_id CHAR(36) NOT NULL,
  modifier_type VARCHAR(50) NOT NULL,
  target VARCHAR(50) NOT NULL DEFAULT '',
  value DECIMAL(14,3) NOT NULL,
  expires_on DATE NOT NULL,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(nation_id,daily_crisis_id,modifier_type,target),
  INDEX idx_crisis_modifier_active(nation_id,expires_on),
  CONSTRAINT fk_crisis_modifier_nation FOREIGN KEY(nation_id) REFERENCES nations(id) ON DELETE CASCADE,
  CONSTRAINT fk_crisis_modifier_daily FOREIGN KEY(daily_crisis_id) REFERENCES daily_crises(id) ON DELETE CASCADE
) ENGINE=InnoDB;
