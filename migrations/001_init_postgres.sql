CREATE TABLE IF NOT EXISTS trade_flows (
    id bigserial PRIMARY KEY,
    year int,
    importer_name text,
    importer_code text,
    exporter_name text,
    exporter_code text,
    commodity text,
    hs_code text,
    trade_value_usd numeric,
    trade_flow text,
    source text,
    created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS event_risk_signals (
    id bigserial PRIMARY KEY,
    country_name text,
    country_code text,
    event_type text,
    event_count int,
    risk_score numeric,
    risk_level text,
    source text,
    created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS macro_scores (
    id bigserial PRIMARY KEY,
    country_name text,
    country_code text,
    macro_score numeric,
    risk_level text,
    source text,
    created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS commodity_prices (
    id bigserial PRIMARY KEY,
    commodity text,
    date_month text,
    price numeric,
    unit text,
    source text,
    created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS dependency_edges (
    id bigserial PRIMARY KEY,
    source_node text,
    target_node text,
    source_type text,
    target_type text,
    relationship_type text,
    weight numeric,
    data_provenance text,
    created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS scenario_runs (
    id bigserial PRIMARY KEY,
    scenario_id text UNIQUE,
    source text,
    commodity text,
    shock_type text,
    drop_percent numeric,
    depth int,
    top_affected_countries jsonb,
    top_affected_sectors jsonb,
    report_json jsonb,
    created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS data_quality_checks (
    id bigserial PRIMARY KEY,
    check_name text,
    status text,
    metric_value numeric,
    details jsonb,
    source text,
    created_at timestamptz DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_trade_flows_importer_commodity
    ON trade_flows (importer_code, commodity);
CREATE INDEX IF NOT EXISTS idx_trade_flows_exporter_commodity
    ON trade_flows (exporter_code, commodity);
CREATE INDEX IF NOT EXISTS idx_trade_flows_year
    ON trade_flows (year);
CREATE INDEX IF NOT EXISTS idx_scenario_runs_created_at
    ON scenario_runs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_event_risk_signals_country_code
    ON event_risk_signals (country_code);
CREATE INDEX IF NOT EXISTS idx_macro_scores_country_code
    ON macro_scores (country_code);
