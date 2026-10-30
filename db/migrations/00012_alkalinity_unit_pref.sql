-- +goose Up
-- The alkalinity display unit "ppm" is now "ppm CaCO3" (the catalog's unit code: ppm alone is
-- ambiguous between kits). Rewrites saved preferences so they stay valid.
UPDATE users SET unit_prefs = jsonb_set(unit_prefs, '{alkalinity}', '"ppm CaCO3"')
WHERE unit_prefs ->> 'alkalinity' = 'ppm';

-- +goose Down
UPDATE users SET unit_prefs = jsonb_set(unit_prefs, '{alkalinity}', '"ppm"')
WHERE unit_prefs ->> 'alkalinity' = 'ppm CaCO3';
