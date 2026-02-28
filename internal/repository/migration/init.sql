DO $$ BEGIN
    CREATE TYPE withdrawal_status AS ENUM ('pending', 'confirmed', 'failed');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Create withdrawals table
CREATE TABLE IF NOT EXISTS withdrawals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL,
    amount DECIMAL(20,8) NOT NULL CHECK (amount > 0),
    currency VARCHAR(10) NOT NULL,
    destination TEXT NOT NULL,
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    status withdrawal_status NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create balances table
CREATE TABLE IF NOT EXISTS balances (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, currency)
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_withdrawals_user_id ON withdrawals(user_id);
CREATE INDEX IF NOT EXISTS idx_withdrawals_status ON withdrawals(status);
CREATE INDEX IF NOT EXISTS idx_withdrawals_created_at ON withdrawals(created_at);
CREATE INDEX IF NOT EXISTS idx_withdrawals_idempotency_key ON withdrawals(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_balances_user_id ON balances(user_id);

-- Insert test data
INSERT INTO balances (user_id, currency, amount) 
VALUES ('user-123', 'USDT', 1000.00)
ON CONFLICT (user_id, currency) DO UPDATE SET amount = EXCLUDED.amount;