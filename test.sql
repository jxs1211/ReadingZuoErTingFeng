SELECT * FROM k8s_pod WHERE NAME LIKE '%cfdeccc0-c0e5-11ee-9b11-0242ac110016%';
SELECT * FROM k8s_deployment WHERE NAME LIKE '%cfdeccc0-c0e5-11ee-9b11-0242ac110016%';
SELECT * FROM k8s_configmap WHERE JSON_EXTRACT(metadata, '$.name') like '%cfdeccc0-c0e5-11ee-9b11-0242ac110016%';
