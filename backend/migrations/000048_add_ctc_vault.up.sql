-- CTC data, end-to-end encrypted in the browser with a passphrase only the
-- CEO knows — never the login password, never sent here in any form. This
-- table only ever holds ciphertext: salt and IVs are not secret (they exist
-- so the same key/nonce never silently gets reused), but the server has no
-- way to read the actual CTC figures, by design. See backend/internal/ctc.
--
-- Single-row table (id is always 1) — one vault, set up once. There is
-- deliberately no passphrase-change/reset path: that would require either
-- storing something that could reconstruct the key (a backdoor) or
-- decrypting with the old passphrase first anyway, which isn't a "reset" at
-- all. Forgetting the passphrase means the data is permanently lost.
CREATE TABLE ctc_vault (
    id BIGINT PRIMARY KEY DEFAULT 1,
    salt TEXT NOT NULL,
    verifier_ciphertext TEXT NOT NULL,
    verifier_iv TEXT NOT NULL,
    data_ciphertext TEXT,
    data_iv TEXT,
    set_up_by BIGINT REFERENCES users(id),
    set_up_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    data_uploaded_by BIGINT REFERENCES users(id),
    data_uploaded_at TIMESTAMPTZ,
    CONSTRAINT ctc_vault_single_row CHECK (id = 1)
);
