-- OpenBadge 초기 스키마
-- OB 3.0 (Open Badges 3.0) 기반

-- Issuer (발급자)
CREATE TABLE IF NOT EXISTS issuers (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    url         VARCHAR(512) NOT NULL,
    email       VARCHAR(255) DEFAULT '',
    description TEXT DEFAULT '',
    image_url   VARCHAR(512) DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- BadgeClass (배지 정의)
CREATE TABLE IF NOT EXISTS badge_classes (
    id           BIGSERIAL PRIMARY KEY,
    issuer_id    BIGINT NOT NULL REFERENCES issuers(id) ON DELETE CASCADE,
    name         VARCHAR(255) NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    image_url    VARCHAR(512) DEFAULT '',
    criteria_url VARCHAR(512) DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_badge_classes_issuer_id ON badge_classes(issuer_id);

-- Recipient (수령자)
CREATE TABLE IF NOT EXISTS recipients (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    email      VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recipients_email ON recipients(email);

-- Assertion (발급 내역)
CREATE TABLE IF NOT EXISTS assertions (
    id                BIGSERIAL PRIMARY KEY,
    badge_class_id    BIGINT NOT NULL REFERENCES badge_classes(id) ON DELETE CASCADE,
    issuer_id         BIGINT NOT NULL REFERENCES issuers(id) ON DELETE CASCADE,
    recipient_id      BIGINT NOT NULL REFERENCES recipients(id) ON DELETE CASCADE,
    issued_on         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at        TIMESTAMPTZ,
    revoked           BOOLEAN NOT NULL DEFAULT FALSE,
    revoked_at        TIMESTAMPTZ,
    revocation_reason TEXT DEFAULT '',
    evidence          TEXT DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_assertions_badge_class_id ON assertions(badge_class_id);
CREATE INDEX IF NOT EXISTS idx_assertions_recipient_id ON assertions(recipient_id);
CREATE INDEX IF NOT EXISTS idx_assertions_issuer_id ON assertions(issuer_id);
