CREATE CHANGEFEED FOR TABLE "purchase"
INTO "pulsar://localhost:6650"
WITH
  envelope = 'row',
  initial_scan = 'no';

CREATE CHANGEFEED FOR TABLE "anomaly"
INTO "pulsar://localhost:6650"
WITH
  envelope = 'row',
  initial_scan = 'no';

CREATE CHANGEFEED FOR TABLE "notification"
INTO "pulsar://localhost:6650"
WITH
  envelope = 'row',
  initial_scan = 'no';