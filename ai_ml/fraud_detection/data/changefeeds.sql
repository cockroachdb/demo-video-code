CREATE CHANGEFEED FOR TABLE "purchase"
INTO "kafka://kafka.default.svc.cluster.local:29092"
WITH
  envelope = 'row',
  initial_scan = 'no',
  kafka_sink_config = '{
    "Flush": {
      "MaxMessages": 1000,
      "Frequency": "100ms"
    },
    "RequiredAcks": "ALL"
  }';

CREATE CHANGEFEED FOR TABLE "anomaly"
INTO "kafka://kafka.default.svc.cluster.local:29092"
WITH
  envelope = 'row',
  initial_scan = 'no',
  kafka_sink_config = '{
    "Flush": {
      "MaxMessages": 1000,
      "Frequency": "100ms"
    },
    "RequiredAcks": "ALL"
  }';

CREATE CHANGEFEED FOR TABLE "notification"
INTO "kafka://kafka.default.svc.cluster.local:29092"
WITH
  envelope = 'row',
  initial_scan = 'no',
  kafka_sink_config = '{
    "Flush": {
      "MaxMessages": 1000,
      "Frequency": "100ms"
    },
    "RequiredAcks": "ALL"
  }';