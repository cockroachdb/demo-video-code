#########################
# ai_ml/fraud_detection #
#########################

REGISTRY := codingconcepts/large-scale-agentic

AGENTS := anomaly reasoning notification

define build_agent
	@echo "Building $(1) agent..."
	@docker buildx build \
		--platform linux/amd64 \
		-f ai_ml/fraud_detection/app/cmd/agents/Dockerfile \
		-t $(REGISTRY)-$(1):$(VERSION)-amd64 \
		--push \
		.

	@docker buildx build \
		--platform linux/arm64 \
		-f ai_ml/fraud_detection/app/cmd/agents/Dockerfile \
		-t $(REGISTRY)-$(1):$(VERSION)-arm64 \
		--push \
		.

	@docker buildx imagetools create \
		-t $(REGISTRY)-$(1):$(VERSION) \
		$(REGISTRY)-$(1):$(VERSION)-amd64 \
		$(REGISTRY)-$(1):$(VERSION)-arm64
endef

build_agent_anomaly: validate_version
	$(call build_agent,anomaly)

build_agent_reasoning: validate_version
	$(call build_agent,reasoning)

build_agent_notification: validate_version
	$(call build_agent,notification)

build_agents_all:
	@$(MAKE) -j3 $(addprefix build_agent_,$(AGENTS))
	@echo "All agents built successfully!"

.PHONY: build_agent_anomaly build_agent_reasoning build_agent_notification build_agents_all

validate_version:
ifndef VERSION
	$(error VERSION is undefined)
endif