-- Products.
INSERT INTO product ("id", "name") VALUES
  ('c28e384e-52d0-4c4a-b1d4-dc55c85977d9', 'Americano'),
  ('0472ea18-a5b2-450c-ab64-03406a6bd27c', 'Cappuccino'),
  ('3b601135-bcde-4e39-9b6f-4218e31e6990', 'Cortado');

-- Market products.
INSERT INTO market_product ("id", "product_id", "market", "amount", "currency") VALUES
  ('66a8c59c-b85f-402d-a566-3c5828e4c20b', 'c28e384e-52d0-4c4a-b1d4-dc55c85977d9', 'de', 3.20, 'eur'),
  ('1dc0d77f-2434-4e6c-a0ac-7041c8d82e80', '0472ea18-a5b2-450c-ab64-03406a6bd27c', 'de', 4.40, 'eur'),
  ('6a050678-e99b-4f23-8251-f13ba09209e9', '3b601135-bcde-4e39-9b6f-4218e31e6990', 'de', 3.40, 'eur'),
  ('99076a81-420f-4309-9d0c-6a3ec5c2ed36', 'c28e384e-52d0-4c4a-b1d4-dc55c85977d9', 'fr', 3.10, 'eur'),
  ('cfcb249d-1c90-4be7-a60c-07a552b51ccb', '0472ea18-a5b2-450c-ab64-03406a6bd27c', 'fr', 4.30, 'eur'),
  ('065b9fc1-1ca8-403e-92db-b87820143b3d', '3b601135-bcde-4e39-9b6f-4218e31e6990', 'fr', 3.30, 'eur'),
  ('69f77aec-7f68-44e4-b86f-6a76cc83c3ef', 'c28e384e-52d0-4c4a-b1d4-dc55c85977d9', 'uk', 3.00, 'gbp'),
  ('0462ccb7-16d7-4c0b-9f1a-153dc5dda004', '0472ea18-a5b2-450c-ab64-03406a6bd27c', 'uk', 4.20, 'gbp'),
  ('569b0028-4e66-448f-b9f0-d27e3a36540a', '3b601135-bcde-4e39-9b6f-4218e31e6990', 'uk', 3.20, 'gbp'),
  ('2dcf61d1-bdd3-4523-a07b-798d14a17ad1', 'c28e384e-52d0-4c4a-b1d4-dc55c85977d9', 'mx', 68.40, 'mxn'),
  ('2c45b8a6-818e-47dc-adc4-e00652cf7c06', '0472ea18-a5b2-450c-ab64-03406a6bd27c', 'mx', 95.80, 'mxn'),
  ('243f4893-5b20-4110-b45b-3f046420f354', '3b601135-bcde-4e39-9b6f-4218e31e6990', 'mx', 72.80, 'mxn'),
  ('b57dfbb2-f94c-49f8-9aac-d185f93ed565', 'c28e384e-52d0-4c4a-b1d4-dc55c85977d9', 'us', 3.84, 'usd'),
  ('d56a2a4a-369b-45e5-90e8-456ea550e13b', '0472ea18-a5b2-450c-ab64-03406a6bd27c', 'us', 5.40, 'usd'),
  ('343963a2-6006-4bb0-a260-6a59fe5397e3', '3b601135-bcde-4e39-9b6f-4218e31e6990', 'us', 4.00, 'usd'),
  ('312d4ac0-063b-4d54-b8a5-39f8d5edd51c', 'c28e384e-52d0-4c4a-b1d4-dc55c85977d9', 'hk', 30.00, 'hkd'),
  ('24bcaeee-01c4-4821-90d4-6dac03e5761e', '0472ea18-a5b2-450c-ab64-03406a6bd27c', 'hk', 42.00, 'hkd'),
  ('43fdef19-ba0f-4146-8493-e3241ec7bb86', '3b601135-bcde-4e39-9b6f-4218e31e6990', 'hk', 32.00, 'hkd'),
  ('06d37900-2677-484e-b03b-bee5fb9a215f', 'c28e384e-52d0-4c4a-b1d4-dc55c85977d9', 'ja', 596, 'jpy'),
  ('ed3f2a6b-ec53-4426-97dc-58afdbb23b0a', '0472ea18-a5b2-450c-ab64-03406a6bd27c', 'ja', 835, 'jpy'),
  ('28fcd099-ab46-4bc6-823c-78aa947a2d24', '3b601135-bcde-4e39-9b6f-4218e31e6990', 'ja', 636, 'jpy');

-- Stock.
INSERT INTO stock ("id", "market_product_id", "crdb_region", "quantity_on_hand", "quantity_on_order") VALUES
  ('bce6221e-773c-4b1b-8d73-57b4008fda2e', '66a8c59c-b85f-402d-a566-3c5828e4c20b', 'aws-eu-central-1', 1000, 100),
  ('968a7e0a-e6f8-4fd6-822c-32eb2a4e26b4', '1dc0d77f-2434-4e6c-a0ac-7041c8d82e80', 'aws-eu-central-1', 1000, 100),
  ('e907dd8d-cf44-4f82-b41f-4a53be31b29c', '6a050678-e99b-4f23-8251-f13ba09209e9', 'aws-eu-central-1', 1000, 100),
  ('f9bc1eb3-0294-478c-b5c0-6b61157b427c', '99076a81-420f-4309-9d0c-6a3ec5c2ed36', 'aws-eu-central-1', 1000, 100),
  ('90a6c529-f2bf-45ad-a5d0-504652ea77b7', 'cfcb249d-1c90-4be7-a60c-07a552b51ccb', 'aws-eu-central-1', 1000, 100),
  ('18fbe269-5d59-4144-82b2-eef9f06c4d5f', '065b9fc1-1ca8-403e-92db-b87820143b3d', 'aws-eu-central-1', 1000, 100),
  ('6dfcc479-2fd4-4187-a03c-22f1b3c952b2', '69f77aec-7f68-44e4-b86f-6a76cc83c3ef', 'aws-eu-central-1', 1000, 100),
  ('fb6b899d-ccee-43ea-b019-57b23366a468', '0462ccb7-16d7-4c0b-9f1a-153dc5dda004', 'aws-eu-central-1', 1000, 100),
  ('ed82a8f8-3c10-4be4-b48e-96c689a6f6ab', '569b0028-4e66-448f-b9f0-d27e3a36540a', 'aws-eu-central-1', 1000, 100),
  ('4de5601a-2c87-4d5d-b7d2-918e1aef6213', '2dcf61d1-bdd3-4523-a07b-798d14a17ad1', 'aws-us-east-1', 1000, 100),
  ('95e29194-ac0e-44e8-be72-39cfe9d9986d', '2c45b8a6-818e-47dc-adc4-e00652cf7c06', 'aws-us-east-1', 1000, 100),
  ('d03f95fb-2037-43f8-9f45-7d1381277817', '243f4893-5b20-4110-b45b-3f046420f354', 'aws-us-east-1', 1000, 100),
  ('c7e52ef7-1fe9-4932-bfba-9e5d73b581ae', 'b57dfbb2-f94c-49f8-9aac-d185f93ed565', 'aws-us-east-1', 1000, 100),
  ('7bf9402d-516e-4090-9f87-f5e545bcfb03', 'd56a2a4a-369b-45e5-90e8-456ea550e13b', 'aws-us-east-1', 1000, 100),
  ('db367767-6557-492f-a8e8-fb2723a3eed2', '343963a2-6006-4bb0-a260-6a59fe5397e3', 'aws-us-east-1', 1000, 100),
  ('66b29ff1-e9fa-4775-b88c-988e488abff6', '312d4ac0-063b-4d54-b8a5-39f8d5edd51c', 'aws-ap-southeast-1', 1000, 100),
  ('37467b4c-02e4-4641-bd39-a684a3785bc6', '24bcaeee-01c4-4821-90d4-6dac03e5761e', 'aws-ap-southeast-1', 1000, 100),
  ('a5491fe4-bff3-400d-895f-2b62023e9157', '43fdef19-ba0f-4146-8493-e3241ec7bb86', 'aws-ap-southeast-1', 1000, 100),
  ('01330780-d06e-4edb-a4de-9d83e9f84a27', '06d37900-2677-484e-b03b-bee5fb9a215f', 'aws-ap-southeast-1', 1000, 100),
  ('e7e03823-355e-49cf-91a8-439b6c6ad5f4', 'ed3f2a6b-ec53-4426-97dc-58afdbb23b0a', 'aws-ap-southeast-1', 1000, 100),
  ('b172b846-49c7-42eb-98ec-69fc009a9801', '28fcd099-ab46-4bc6-823c-78aa947a2d24', 'aws-ap-southeast-1', 1000, 100);

-- Region market.
INSERT INTO region_market ("region", "market")
  SELECT DISTINCT crdb_region, market FROM market_product;