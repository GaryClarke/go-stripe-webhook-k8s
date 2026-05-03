-- Downstream service: initial PostgreSQL schema + fixture data
-- Run after: CREATE DATABASE downstream; (or use compose defaults)
-- Requires: pgcrypto for gen_random_uuid() — usually available as CREATE EXTENSION

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------------------------------------------------------------------------
-- customers
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stripe_customer_id TEXT NOT NULL UNIQUE,
    email TEXT,
    name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- invoices (Stripe amounts in minor units; currency lowercase ISO 4217)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stripe_invoice_id TEXT NOT NULL UNIQUE,
    customer_id UUID NOT NULL REFERENCES customers (id) ON DELETE RESTRICT,
    status TEXT NOT NULL,
    amount_due BIGINT NOT NULL,
    amount_paid BIGINT NOT NULL,
    currency CHAR(3) NOT NULL,
    stripe_created TIMESTAMPTZ NOT NULL,
    due_date TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_invoices_customer_id ON invoices (customer_id);
CREATE INDEX IF NOT EXISTS idx_invoices_stripe_created ON invoices (stripe_created DESC);
CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices (status);

-- ---------------------------------------------------------------------------
-- Seed / fixture data (stable UUIDs for predictable API examples)
-- ---------------------------------------------------------------------------
INSERT INTO customers (id, stripe_customer_id, email, name, created_at, updated_at)
VALUES
    (
        'a0000001-0000-4000-8000-000000000001'::uuid,
        'cus_UB6AMU86WjR4K3',
        'ada@example.com',
        'Ada Lovelace',
        '2026-01-15T10:00:00Z'::timestamptz,
        '2026-01-15T10:00:00Z'::timestamptz
    ),
    (
        'a0000002-0000-4000-8000-001000000001'::uuid,
        'cus_DEMO02example',
        'babbage@example.com',
        'Charles Babbage',
        '2026-02-01T14:30:00Z'::timestamptz,
        '2026-02-01T14:30:00Z'::timestamptz
    )
ON CONFLICT (stripe_customer_id) DO NOTHING;

INSERT INTO invoices (
    id,
    stripe_invoice_id,
    customer_id,
    status,
    amount_due,
    amount_paid,
    currency,
    stripe_created,
    due_date,
    paid_at,
    created_at,
    updated_at
)
VALUES
    (
        'b1000001-0000-4000-8000-000000000001'::uuid,
        'in_1TCjy5Iq4hctS9aMF2VnM9wT',
        'a0000001-0000-4000-8000-000000000001'::uuid,
        'paid',
        2000,
        2000,
        'usd',
        '2026-03-10T09:00:00Z'::timestamptz,
        NULL,
        '2026-03-10T09:02:00Z'::timestamptz,
        '2026-03-10T09:05:00Z'::timestamptz,
        '2026-03-10T09:05:00Z'::timestamptz
    ),
    (
        'b1000002-0000-4000-8000-000000000002'::uuid,
        'in_DEMO_open_invoice',
        'a0000001-0000-4000-8000-000000000001'::uuid,
        'open',
        4999,
        0,
        'gbp',
        '2026-03-20T12:00:00Z'::timestamptz,
        '2026-04-01T00:00:00Z'::timestamptz,
        NULL,
        '2026-03-20T12:10:00Z'::timestamptz,
        '2026-03-20T12:10:00Z'::timestamptz
    ),
    (
        'b1000003-0000-4000-8000-000000000003'::uuid,
        'in_DEMO_paid_gbp',
        'a0000002-0000-4000-8000-001000000001'::uuid,
        'paid',
        1500,
        1500,
        'gbp',
        '2026-03-22T08:00:00Z'::timestamptz,
        NULL,
        '2026-03-22T08:15:00Z'::timestamptz,
        '2026-03-22T08:20:00Z'::timestamptz,
        '2026-03-22T08:20:00Z'::timestamptz
    )
ON CONFLICT (stripe_invoice_id) DO NOTHING;
