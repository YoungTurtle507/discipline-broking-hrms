DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE code = 'candidates.manage');
DELETE FROM permissions WHERE code = 'candidates.manage';
DROP TABLE IF EXISTS candidates;
DROP TYPE IF EXISTS candidate_status;
DROP TYPE IF EXISTS candidate_notice_period;
DROP TYPE IF EXISTS candidate_experience_type;
