CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE doctors (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE doctor_schedules (
    id          INTEGER  GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    doctor_id   UUID     NOT NULL REFERENCES doctors (id) ON DELETE CASCADE,
    day_of_week SMALLINT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6), -- 0 = Sunday
    start_time  TIME     NOT NULL,
    end_time    TIME     NOT NULL,
    CHECK (end_time > start_time),

    EXCLUDE USING gist (
        doctor_id WITH =,
        day_of_week WITH =,
        tsrange(DATE '2000-01-01' + start_time, DATE '2000-01-01' + end_time) WITH &&
    )
);

CREATE TABLE appointments (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    doctor_id           UUID        NOT NULL REFERENCES doctors (id),
    patient_id          TEXT        NOT NULL CHECK (char_length(btrim(patient_id)) BETWEEN 1 AND 64),
    start_time          TIMESTAMPTZ NOT NULL,
    end_time            TIMESTAMPTZ NOT NULL,
    status              TEXT        NOT NULL DEFAULT 'CONFIRMED'
                                    CHECK (status IN ('CONFIRMED', 'CANCELLED')),
    cancellation_reason TEXT        CHECK (char_length(cancellation_reason) <= 500),
    late_cancellation   BOOLEAN     NOT NULL DEFAULT false,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    CHECK (end_time = start_time + INTERVAL '30 minutes'),

    CONSTRAINT appointments_cancellation_consistency CHECK (
        (status = 'CONFIRMED'
            AND cancellation_reason IS NULL
            AND late_cancellation = false)
        OR
        (status = 'CANCELLED'
            AND cancellation_reason IS NOT NULL
            AND char_length(btrim(cancellation_reason)) > 0)
    )
);

-- Prevents teh race conidtion of double-booking
CREATE UNIQUE INDEX uniq_active_slot
    ON appointments (doctor_id, start_time)
    WHERE status = 'CONFIRMED';

-- patient cannot have two confirmed appointments at the same time, with any doc.
CREATE UNIQUE INDEX uniq_patient_slot
    ON appointments (patient_id, start_time)
    WHERE status = 'CONFIRMED';