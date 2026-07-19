CREATE TABLE IF NOT EXISTS custom_trade_flows (
    id bigserial PRIMARY KEY,
    dataset_name text,
    importer text,
    commodity text,
    supplier text,
    value_usd numeric,
    created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS custom_concentration_results (
    id bigserial PRIMARY KEY,
    dataset_name text,
    importer text,
    commodity text,
    total_value_usd numeric,
    supplier_count int,
    top_supplier text,
    top_supplier_share numeric,
    hhi numeric,
    concentration_risk text,
    created_at timestamptz DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_custom_trade_flows_dataset
    ON custom_trade_flows (dataset_name);
CREATE INDEX IF NOT EXISTS idx_custom_concentration_dataset
    ON custom_concentration_results (dataset_name);
