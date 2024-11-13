deploy_app:
	cp go.* app
	(cd app && docker build -t app .)
	docker tag app:latest localhost:9090/app:latest
	docker push localhost:9090/app:latest
	kubectl apply -f manifests/app/deployment.yaml