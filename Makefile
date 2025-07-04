#
# Copyright 2024 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

### GitHub Actions tasks
generate:
	python3 app/templates/generate.py

validate_generate:
	python3 app/templates/generate.py --validate


### Local development helpers
apply:
	terraform -chdir=infra apply -auto-approve

build_latest_images:
	gcloud builds submit --config build/images.cloudbuild.yaml --substitutions TAG_NAME=latest


### CFT Standard
# These need to be here, as this is what is detected in .github/workflows/lint.yaml

SHELL := /usr/bin/env bash

DOCKER_TAG_VERSION_DEVELOPER_TOOLS := 1.25
DOCKER_IMAGE_DEVELOPER_TOOLS := cft/developer-tools
REGISTRY_URL := gcr.io/cloud-foundation-cicd
ENABLE_BPMETADATA := 1
export ENABLE_BPMETADATA

# Enter docker container for local development
.PHONY: docker_run
docker_run:
	docker run --rm -it \
		-e SERVICE_ACCOUNT_JSON \
		-v "$(CURDIR)":/workspace \
		$(REGISTRY_URL)/${DOCKER_IMAGE_DEVELOPER_TOOLS}:${DOCKER_TAG_VERSION_DEVELOPER_TOOLS} \
		/bin/bash

# Execute prepare tests within the docker container
.PHONY: docker_test_prepare
docker_test_prepare:
	docker run --rm -it \
		-e SERVICE_ACCOUNT_JSON \
		-e TF_VAR_org_id \
		-e TF_VAR_folder_id \
		-e TF_VAR_billing_account \
		-v "$(CURDIR)":/workspace \
		$(REGISTRY_URL)/${DOCKER_IMAGE_DEVELOPER_TOOLS}:${DOCKER_TAG_VERSION_DEVELOPER_TOOLS} \
		/usr/local/bin/execute_with_credentials.sh prepare_environment

# Clean up test environment within the docker container
.PHONY: docker_test_cleanup
docker_test_cleanup:
	docker run --rm -it \
		-e SERVICE_ACCOUNT_JSON \
		-e TF_VAR_org_id \
		-e TF_VAR_folder_id \
		-e TF_VAR_billing_account \
		-v "$(CURDIR)":/workspace \
		$(REGISTRY_URL)/${DOCKER_IMAGE_DEVELOPER_TOOLS}:${DOCKER_TAG_VERSION_DEVELOPER_TOOLS} \
		/usr/local/bin/execute_with_credentials.sh cleanup_environment

# Execute integration tests within the docker container
.PHONY: docker_test_integration
docker_test_integration:
	docker run --rm -it \
		-e SERVICE_ACCOUNT_JSON \
		-v "$(CURDIR)":/workspace \
		$(REGISTRY_URL)/${DOCKER_IMAGE_DEVELOPER_TOOLS}:${DOCKER_TAG_VERSION_DEVELOPER_TOOLS} \
		/usr/local/bin/test_integration.sh

# Execute lint tests within the docker container
.PHONY: docker_test_lint
docker_test_lint:
	docker run --rm -it \
		-e ENABLE_BPMETADATA \
		-e EXCLUDE_LINT_DIRS \
		-v "$(CURDIR)":/workspace \
		$(REGISTRY_URL)/${DOCKER_IMAGE_DEVELOPER_TOOLS}:${DOCKER_TAG_VERSION_DEVELOPER_TOOLS} \
		/usr/local/bin/test_lint.sh

# Generate documentation
.PHONY: docker_generate_docs
docker_generate_docs:
	docker run --rm -it \
		-e ENABLE_BPMETADATA \
		-v "$(dir ${CURDIR})":/workspace \
		$(REGISTRY_URL)/${DOCKER_IMAGE_DEVELOPER_TOOLS}:${DOCKER_TAG_VERSION_DEVELOPER_TOOLS} \
		/bin/bash -c 'source /usr/local/bin/task_helper_functions.sh && generate_docs "-d -p infra"'

# Alias for backwards compatibility
.PHONY: generate_docs
generate_docs: docker_generate_docs

