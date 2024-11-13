CREATE TABLE example (
  uuid_column UUID NOT NULL,
  date_column DATE NOT NULL,
  timestamp_column TIMESTAMPTZ NOT NULL,
  int_column INT NOT NULL,
  serial_column SERIAL NOT NULL,
  string_column STRING NOT NULL,

  PRIMARY KEY (uuid_column),
  INDEX (date_column),
  INDEX (timestamp_column),
  INDEX (int_column),
  INDEX (serial_column),
  INDEX (string_column),
  INDEX (int_column, string_column),
  INDEX (string_column) STORING (serial_column)
);

ALTER TABLE example CONFIGURE ZONE USING
  range_min_bytes = 0,
  range_max_bytes = 1048576;

SET CLUSTER SETTING sql.show_ranges_deprecated_behavior.enabled = false;
