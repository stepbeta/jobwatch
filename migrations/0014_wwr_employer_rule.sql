-- 0014_wwr_employer_rule: We Work Remotely titles are "Employer: Role".
--
-- The RSS adapter now supports a declarative employer rule. This
-- configures it for the registered We Work Remotely feed so the adapter
-- emits the employer as company evidence and strips the prefix from the
-- title. The rule is added only when the source has no employer rule
-- yet, so a user edit is not overwritten; every other config key is kept.
--
-- This is a data-only, additive migration.

UPDATE sources
SET config_json = json_set(config_json, '$.employer', json('{"separator":":"}'))
WHERE url = 'https://weworkremotely.com/remote-jobs.rss'
  AND json_extract(config_json, '$.employer') IS NULL;
