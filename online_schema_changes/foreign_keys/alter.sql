ALTER TABLE pet ADD COLUMN country STRING;
UPDATE pet SET country = p.country
FROM person p
WHERE person_id = p.id;

ALTER TABLE pet ALTER COLUMN country SET NOT NULL;

ALTER TABLE person ALTER PRIMARY KEY USING COLUMNS (country, id);
ALTER TABLE pet ALTER PRIMARY KEY USING COLUMNS (country, id);

ALTER TABLE pet ADD CONSTRAINT pet_person_id_country_fkey FOREIGN KEY (country, person_id) REFERENCES person(country, id);
ALTER TABLE pet DROP CONSTRAINT pet_person_id_fkey;