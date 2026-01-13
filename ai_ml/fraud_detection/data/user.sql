-- Insert a known customer.
INSERT INTO customer(id, email, phone, preferred_contact) VALUES(
  'c7fc4006-3f39-4baf-ad93-5870f3ec27ec',
  'anomalies@testing.com',
  '+441234567890',
  'email'
);

-- Insert regular purchases for them
INSERT INTO purchase(customer_id, amount, location, ts)
  SELECT 
    'c7fc4006-3f39-4baf-ad93-5870f3ec27ec',
    ROUND((random() * 90 + 10)::numeric, 2),
    ST_GeomFromText('POINT(' || 
      ROUND((random() * 0.9 - 0.45)::numeric, 4) || ' ' || 
      ROUND((random() * 0.5 + 51.2)::numeric, 4) || 
    ')'),
    '2025-01-01T08:00:00Z'::timestamp + (n || ' minutes')::interval
  FROM generate_series(1, 1000) AS n;