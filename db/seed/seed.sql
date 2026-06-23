-- Provider seed data
INSERT INTO providers (id, name, display_name, adapter_type, official_api_base_url, oauth_endpoint) VALUES
('a0000000-0000-0000-0000-000000000001', 'openai', 'OpenAI', 'openai', 'https://api.openai.com/v1', 'https://auth.openai.com/oauth/token'),
('a0000000-0000-0000-0000-000000000002', 'anthropic', 'Anthropic', 'claude', 'https://api.anthropic.com/v1', 'https://auth.anthropic.com/oauth/token'),
('a0000000-0000-0000-0000-000000000003', 'gemini', 'Google Gemini', 'gemini', 'https://generativelanguage.googleapis.com/v1beta', NULL)
ON CONFLICT DO NOTHING;

-- Default organization (for development)
INSERT INTO organizations (id, name, slug) VALUES
('b0000000-0000-0000-0000-000000000001', 'Default Org', 'default')
ON CONFLICT DO NOTHING;
