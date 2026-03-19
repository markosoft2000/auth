INSERT INTO apps (id, name, secret)
VALUES (1, 'testApp', 'testSecret')
ON CONFLICT DO NOTHING;