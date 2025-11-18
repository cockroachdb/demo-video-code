CREATE CHANGEFEED FOR TABLE "purchase"
INTO "pulsar://pulsar.default.svc.cluster.local:6650"
WITH
  envelope = 'row',
  initial_scan = 'no',
  unordered;

CREATE CHANGEFEED FOR TABLE "anomaly"
INTO "pulsar://pulsar.default.svc.cluster.local:6650"
WITH
  envelope = 'row',
  initial_scan = 'no',
  unordered;

CREATE CHANGEFEED FOR TABLE "notification"
INTO "pulsar://pulsar.default.svc.cluster.local:6650"
WITH
  envelope = 'row',
  initial_scan = 'no',
  unordered;