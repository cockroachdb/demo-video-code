INSERT INTO customer ("id", "email", "gender", "date_of_birth", "location") VALUES
  ('76d33bad-8e43-47d9-ac7f-4e10463d8671','abc@acme.com','male','1988-07-14',st_point(51.54132696656601, -0.14507005496345876));

UPSERT INTO product ("id", "name", "price") VALUES
  ('000d9be3-b357-48b1-8f3b-53472f186faf', 'p-000', 10.99),
  ('11178714-7e53-4d23-a2cf-30cd1984f39b', 'p-111', 20.99),
  ('22234a61-2858-4b75-a2e8-bb80d4de1a93', 'p-222', 30.99),
  ('3334a972-12cd-4573-af23-16db9f8e9f40', 'p-333', 40.99),
  ('44419600-100f-4935-819f-ebca5b9464a4', 'p-444', 50.99),
  ('555349bc-f1d0-48d5-8ee3-5dc6ac619859', 'p-555', 50.99);

INSERT INTO purchase ("id", "customer_id", "total") VALUES
  ('60916d80-54ce-479a-a828-f2dd9136ae98', '76d33bad-8e43-47d9-ac7f-4e10463d8671', 1.99),
  ('f93b396e-814d-43f1-9420-43597c329941', '76d33bad-8e43-47d9-ac7f-4e10463d8671', 2.99),
  ('3c9fe8fa-5c40-4a8e-8ad2-d1db2c67a218', '76d33bad-8e43-47d9-ac7f-4e10463d8671', 3.99);

INSERT INTO purchase_item ("purchase_id", "product_id", "quantity") VALUES
  ('60916d80-54ce-479a-a828-f2dd9136ae98', '000d9be3-b357-48b1-8f3b-53472f186faf', 1),
  ('f93b396e-814d-43f1-9420-43597c329941', '000d9be3-b357-48b1-8f3b-53472f186faf', 2),
  ('f93b396e-814d-43f1-9420-43597c329941', '11178714-7e53-4d23-a2cf-30cd1984f39b', 3),
  ('3c9fe8fa-5c40-4a8e-8ad2-d1db2c67a218', '11178714-7e53-4d23-a2cf-30cd1984f39b', 4),
  ('3c9fe8fa-5c40-4a8e-8ad2-d1db2c67a218', '22234a61-2858-4b75-a2e8-bb80d4de1a93', 5),
  ('3c9fe8fa-5c40-4a8e-8ad2-d1db2c67a218', '3334a972-12cd-4573-af23-16db9f8e9f40', 6);