-- Demo data: 5 docs
--Doc IDs are fixed so docs and tests can reference them.
BEGIN;

INSERT INTO doctors (id, name) VALUES
    ('11111111-1111-4111-8111-111111111111', 'Dr. Sifuna Edwin'),
    ('22222222-2222-4222-8222-222222222222', 'Dr. Rigathi Wamunyoro'),
    ('33333333-3333-4333-8333-333333333333', 'Dr. Edwin Kariuki'),
    ('44444444-4444-4444-8444-444444444444', 'Dr. Kasongo Wantam'),
    ('55555555-5555-4555-8555-555555555555', 'Dr. Willy Pozze')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

DELETE FROM doctor_schedules
WHERE doctor_id IN (
    '11111111-1111-4111-8111-111111111111',
    '22222222-2222-4222-8222-222222222222',
    '33333333-3333-4333-8333-333333333333',
    '44444444-4444-4444-8444-444444444444',
    '55555555-5555-4555-8555-555555555555'
);

-- Dr. Sifuna Edwin: Mon-Fri, 08:00-12:00 and 13:00-17:00
INSERT INTO doctor_schedules (doctor_id, day_of_week, start_time, end_time)
SELECT '11111111-1111-4111-8111-111111111111', d, s.start_time::time, s.end_time::time
FROM generate_series(1, 5) AS d,
     (VALUES ('08:00', '12:00'), ('13:00', '17:00')) AS s (start_time, end_time);

-- Dr. Rigathi Wamunyoro: Mon-Fri, 09:00-13:00
INSERT INTO doctor_schedules (doctor_id, day_of_week, start_time, end_time)
SELECT '22222222-2222-4222-8222-222222222222', d, '09:00', '13:00'
FROM generate_series(1, 5) AS d;

-- Dr. Edwin Kariuki: Mon, Wed, Fri, 08:00-12:00 and 14:00-18:00
INSERT INTO doctor_schedules (doctor_id, day_of_week, start_time, end_time)
SELECT '33333333-3333-4333-8333-333333333333', d, s.start_time::time, s.end_time::time
FROM (VALUES (1), (3), (5)) AS days (d),
     (VALUES ('08:00', '12:00'), ('14:00', '18:00')) AS s (start_time, end_time);

-- Dr. Kasongo Wantam: Tue and Thu 07:00-15:00, Sat 09:00-13:00
INSERT INTO doctor_schedules (doctor_id, day_of_week, start_time, end_time) VALUES
    ('44444444-4444-4444-8444-444444444444', 2, '07:00', '15:00'),
    ('44444444-4444-4444-8444-444444444444', 4, '07:00', '15:00'),
    ('44444444-4444-4444-8444-444444444444', 6, '09:00', '13:00');

-- Dr. Willy Pozze: Mon-Sat, 10:00-16:00
INSERT INTO doctor_schedules (doctor_id, day_of_week, start_time, end_time)
SELECT '55555555-5555-4555-8555-555555555555', d, '10:00', '16:00'
FROM generate_series(1, 6) AS d;

COMMIT;