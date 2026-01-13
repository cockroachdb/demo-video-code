.PHONY: build_agent_anomaly build_agent_reasoning build_agent_notification build_agents_all

validate_version:
ifndef VERSION
	$(error VERSION is undefined)
endif

validate_registry:
ifndef REGISTRY
	$(error REGISTRY is undefined)
endif

#########################
# ai_ml/fraud_detection #
#########################

build_agent: validate_version validate_registry
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ./app/cmd/agents/agent ./app/cmd/agents
	@docker buildx build \
		--platform linux/amd64 \
		-t ${REGISTRY}:${VERSION}-amd64 \
		--push \
		./app/cmd/agents
	
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o ./app/cmd/agents/agent ./app/cmd/agents
	@docker buildx build \
		--platform linux/arm64 \
		-t ${REGISTRY}:${VERSION}-arm64 \
		--push  \
		./app/cmd/agents

	@rm ./app/cmd/agents/agent

	@docker buildx imagetools create \
		-t ${REGISTRY}:${VERSION} \
		${REGISTRY}:${VERSION}-amd64 \
		${REGISTRY}:${VERSION}-arm64