INSERT INTO apps (id, name, secret)
VALUES (1, 'testApp', 'testSecret')
ON CONFLICT DO NOTHING;

INSERT INTO admins (id, is_admin)
VALUES (1, TRUE)
ON CONFLICT DO NOTHING;