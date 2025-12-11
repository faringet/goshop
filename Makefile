# ================== ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ ==================

KIND_CLUSTER  ?= goshop
K8S_NAMESPACE ?= goshop
K8S_DIR       ?= k8s
DOCKER_COMPOSE ?= docker compose
DC_PROJECT     ?= goshop

.PHONY: \
	k8s-ingress-nginx-install k8s-ingress-nginx-wait k8s-ingress-apply k8s-ingress-bootstrap \
	k8s-ingress-nginx-download k8s-portforward-ingress \
	users-image users-kind-load users-build-and-load \
	k8s-users-apply k8s-users-status k8s-users-logs \
	orders-image orders-kind-load orders-build-and-load \
	k8s-orders-apply k8s-orders-status k8s-orders-logs \
	outboxer-image outboxer-kind-load outboxer-build-and-load \
	k8s-outboxer-apply k8s-outboxer-status k8s-outboxer-logs \
	payments-image payments-kind-load payments-build-and-load \
	k8s-payments-apply k8s-payments-status k8s-payments-logs \
	gateway-image gateway-kind-load gateway-build-and-load \
	k8s-gateway-apply k8s-gateway-status k8s-gateway-logs \
	opsassistant-image opsassistant-kind-load opsassistant-build-and-load \
	k8s-opsassistant-apply k8s-opsassistant-status k8s-opsassistant-logs \
	k8s-kind-create k8s-kind-delete k8s-build-and-load-all k8s-apply-all k8s-bootstrap \
	k8s-status-all \
	k8s-redeploy-users k8s-redeploy-orders k8s-redeploy-payments \
	k8s-redeploy-outboxer k8s-redeploy-gateway k8s-redeploy-opsassistant \
	k8s-clean k8s-redeploy-all \
	docker-all-up docker-all-down docker-infra-up docker-infra-down \
	dev-docker-only dev-docker-only-down dev-hybrid dev-hybrid-down


# ================== DOCKER COMPOSE (DEV-СЦЕНАРИИ) ==================

# Все композы: инфраструктура + сервисы + логи
DOCKER_FILES_ALL = \
	-f ./docker-compose.infra.yml \
	-f ./docker-compose.ollama.yml \
	-f ./docker-compose.users.yml \
	-f ./docker-compose.payments.yml \
	-f ./docker-compose.orders.yml \
	-f ./docker-compose.outboxer.yml \
	-f ./docker-compose.gateway.yml \
	-f ./docker-compose.opsassistant.yml \
	-f ./docker-compose.logs.yml

# Только инфраструктура + логи + ollama (для гибридного сценария)
DOCKER_FILES_INFRA = \
	-f ./docker-compose.infra.yml \
	-f ./docker-compose.ollama.yml \
	-f ./docker-compose.logs.yml

# Всё в Docker: infra + users + orders + payments + outboxer + gateway + opsassistant + logs
docker-all-up:
	$(DOCKER_COMPOSE) -p $(DC_PROJECT) \
		$(DOCKER_FILES_ALL) \
		up -d --build

docker-all-down:
	$(DOCKER_COMPOSE) -p $(DC_PROJECT) \
		$(DOCKER_FILES_ALL) \
		down -v

# Удобные алиасы
dev-docker-only: docker-all-up
	@echo "Dev mode: все (infra + сервисы + логи) запущено в Docker проекте $(DC_PROJECT)."

dev-docker-only-down: docker-all-down
	@echo "Dev mode: все контейнеры Docker для проекта $(DC_PROJECT) остановлены и удалены."

# Только инфраструктура: БД, Kafka, Ollama, логи и т.п.
docker-infra-up:
	$(DOCKER_COMPOSE) -p $(DC_PROJECT) \
		$(DOCKER_FILES_INFRA) \
		up -d --build

docker-infra-down:
	$(DOCKER_COMPOSE) -p $(DC_PROJECT) \
		$(DOCKER_FILES_INFRA) \
		down -v



dev-hybrid:
	@echo "1) Bringing up Docker infra (DB, Kafka, Ollama, logs)..."
	$(MAKE) docker-infra-up
	$(MAKE) docker-migrate-all
	@echo "2) Creating kind cluster $(KIND_CLUSTER) (if not exists)..."
	-$(MAKE) k8s-kind-create
	@echo "3) Building and loading all images into kind..."
	$(MAKE) k8s-build-and-load-all
	@echo "4) Applying all k8s manifests (deployments/services)..."
	$(MAKE) k8s-apply-all
	@echo "5) Installing ingress-nginx and app ingresses..."
	$(MAKE) k8s-ingress-bootstrap
	@echo "Hybrid dev mode: infra/logs in Docker, services in kind cluster $(KIND_CLUSTER)."


dev-hybrid-down:
	@echo "1) Cleaning namespace $(K8S_NAMESPACE) in k8s..."
	$(MAKE) k8s-clean
	@echo "2) Stopping Docker infra for project $(DC_PROJECT)..."
	$(MAKE) docker-infra-down
	@echo "Hybrid dev mode: k8s namespace $(K8S_NAMESPACE) cleaned, Docker infra stopped."



# ================== КЛАСТЕР KIND ==================

# Поднять кластер kind с дефолтной конфигурацией
k8s-kind-create:
	kind create cluster --name $(KIND_CLUSTER)

# Снести кластер kind
k8s-kind-delete:
	kind delete cluster --name $(KIND_CLUSTER)


# ================== INGRESS-NGINX ==================

# 1) Скачиваем манифест ingress-nginx под kind в локальный файл
k8s-ingress-nginx-download:
	@if not exist "$(K8S_DIR)\infra" mkdir "$(K8S_DIR)\infra"
	curl -L "https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml" \
		-o "$(K8S_DIR)/infra/ingress-nginx-kind.yaml"

# 2) Ставим ingress-nginx в кластер из локального файла
k8s-ingress-nginx-install: k8s-ingress-nginx-download
	kubectl apply -f $(K8S_DIR)/infra/ingress-nginx-kind.yaml

# 3) Ждём, пока контроллер поднимется
k8s-ingress-nginx-wait:
	kubectl wait --namespace ingress-nginx \
	  --for=condition=ready pod \
	  --selector=app.kubernetes.io/component=controller \
	  --timeout=120s

# 4) Применяем все ingress'ы приложения
k8s-ingress-apply:
	kubectl apply -f $(K8S_DIR)/users-ingress.yaml
	kubectl apply -f $(K8S_DIR)/gateway-ingress.yaml
	kubectl apply -f $(K8S_DIR)/opsassistant-ingress.yaml

# Полный цикл для ingress: поставить контроллер + дождаться + применить ingress'ы
k8s-ingress-bootstrap: k8s-ingress-nginx-install k8s-ingress-nginx-wait k8s-ingress-apply
	@echo "Ingress-nginx и ingress-правила для users/gateway/opsassistant развернуты."

# Удобный таргет для проброса 8080 -> ingress-nginx (для локальной работы)
k8s-portforward-ingress:
	kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 8080:80


# ================== USERS ==================

USERS_IMAGE ?= goshop-users
USERS_TAG   ?= dev

users-image:
	docker build -f services/users/Dockerfile -t $(USERS_IMAGE):$(USERS_TAG) --target users-app .

users-kind-load: users-image
	kind load docker-image $(USERS_IMAGE):$(USERS_TAG) --name $(KIND_CLUSTER)

users-build-and-load: users-kind-load
	@echo "users image $(USERS_IMAGE):$(USERS_TAG) loaded into kind cluster $(KIND_CLUSTER)"

k8s-users-apply:
	kubectl apply -f $(K8S_DIR)/namespace-goshop.yaml
	kubectl apply -f $(K8S_DIR)/users-configmap.yaml
	kubectl apply -f $(K8S_DIR)/users-deployment.yaml
	kubectl apply -f $(K8S_DIR)/users-service.yaml

k8s-users-status:
	kubectl get pods,svc -n $(K8S_NAMESPACE) -l app=users

k8s-users-logs:
	kubectl logs -n $(K8S_NAMESPACE) deploy/users --tail=100


# ================== ORDERS ==================

ORDERS_IMAGE ?= goshop-orders
ORDERS_TAG   ?= dev

orders-image:
	docker build -f services/orders/Dockerfile -t $(ORDERS_IMAGE):$(ORDERS_TAG) --target orders-app .

orders-kind-load: orders-image
	kind load docker-image $(ORDERS_IMAGE):$(ORDERS_TAG) --name $(KIND_CLUSTER)

orders-build-and-load: orders-kind-load
	@echo "orders image $(ORDERS_IMAGE):$(ORDERS_TAG) loaded into kind cluster $(KIND_CLUSTER)"

k8s-orders-apply:
	kubectl apply -f $(K8S_DIR)/orders-configmap.yaml
	kubectl apply -f $(K8S_DIR)/orders-deployment.yaml
	kubectl apply -f $(K8S_DIR)/orders-service.yaml

k8s-orders-status:
	kubectl get pods,svc -n $(K8S_NAMESPACE) -l app=orders

k8s-orders-logs:
	kubectl logs -n $(K8S_NAMESPACE) deploy/orders --tail=100


# ================== OUTBOXER ==================

OUTBOXER_IMAGE ?= goshop-outboxer
OUTBOXER_TAG   ?= dev

outboxer-image:
	docker build -f services/outboxer/Dockerfile -t $(OUTBOXER_IMAGE):$(OUTBOXER_TAG) --target outboxer-app .

outboxer-kind-load: outboxer-image
	kind load docker-image $(OUTBOXER_IMAGE):$(OUTBOXER_TAG) --name $(KIND_CLUSTER)

outboxer-build-and-load: outboxer-kind-load
	@echo "outboxer image $(OUTBOXER_IMAGE):$(OUTBOXER_TAG) loaded into kind cluster $(KIND_CLUSTER)"

k8s-outboxer-apply:
	kubectl apply -f $(K8S_DIR)/outboxer-configmap.yaml
	kubectl apply -f $(K8S_DIR)/outboxer-deployment.yaml

k8s-outboxer-status:
	kubectl get pods -n $(K8S_NAMESPACE) -l app=outboxer

k8s-outboxer-logs:
	kubectl logs -n $(K8S_NAMESPACE) deploy/outboxer --tail=100


# ================== PAYMENTS ==================

PAYMENTS_IMAGE ?= goshop-payments
PAYMENTS_TAG   ?= dev

payments-image:
	docker build -f services/payments/Dockerfile -t $(PAYMENTS_IMAGE):$(PAYMENTS_TAG) --target payments-app .

payments-kind-load: payments-image
	kind load docker-image $(PAYMENTS_IMAGE):$(PAYMENTS_TAG) --name $(KIND_CLUSTER)

payments-build-and-load: payments-kind-load
	@echo "payments image $(PAYMENTS_IMAGE):$(PAYMENTS_TAG) loaded into kind cluster $(KIND_CLUSTER)"

k8s-payments-apply:
	kubectl apply -f $(K8S_DIR)/payments-configmap.yaml
	kubectl apply -f $(K8S_DIR)/payments-deployment.yaml
	kubectl apply -f $(K8S_DIR)/payments-service.yaml

k8s-payments-status:
	kubectl get pods,svc -n $(K8S_NAMESPACE) -l app=payments

k8s-payments-logs:
	kubectl logs -n $(K8S_NAMESPACE) deploy/payments --tail=100


# ================== GATEWAY ==================

GATEWAY_IMAGE ?= goshop-gateway
GATEWAY_TAG   ?= dev

gateway-image:
	docker build -f services/gateway/Dockerfile -t $(GATEWAY_IMAGE):$(GATEWAY_TAG) --target gateway-app .

gateway-kind-load: gateway-image
	kind load docker-image $(GATEWAY_IMAGE):$(GATEWAY_TAG) --name $(KIND_CLUSTER)

gateway-build-and-load: gateway-kind-load
	@echo "gateway image $(GATEWAY_IMAGE):$(GATEWAY_TAG) loaded into kind cluster $(KIND_CLUSTER)"

k8s-gateway-apply:
	kubectl apply -f $(K8S_DIR)/gateway-configmap.yaml
	kubectl apply -f $(K8S_DIR)/gateway-deployment.yaml
	kubectl apply -f $(K8S_DIR)/gateway-service.yaml

k8s-gateway-status:
	kubectl get pods,svc -n $(K8S_NAMESPACE) -l app=gateway

k8s-gateway-logs:
	kubectl logs -n $(K8S_NAMESPACE) deploy/gateway --tail=100


# ================== OPSASSISTANT ==================

OPSASSISTANT_IMAGE ?= goshop-opsassistant
OPSASSISTANT_TAG   ?= dev

opsassistant-image:
	docker build -f services/opsassistant/Dockerfile -t $(OPSASSISTANT_IMAGE):$(OPSASSISTANT_TAG) --target opsassistant-app .

opsassistant-kind-load: opsassistant-image
	kind load docker-image $(OPSASSISTANT_IMAGE):$(OPSASSISTANT_TAG) --name $(KIND_CLUSTER)

opsassistant-build-and-load: opsassistant-kind-load
	@echo "opsassistant image $(OPSASSISTANT_IMAGE):$(OPSASSISTANT_TAG) loaded into kind cluster $(KIND_CLUSTER)"

k8s-opsassistant-apply:
	kubectl apply -f $(K8S_DIR)/opsassistant-configmap.yaml
	kubectl apply -f $(K8S_DIR)/opsassistant-deployment.yaml
	kubectl apply -f $(K8S_DIR)/opsassistant-service.yaml

k8s-opsassistant-status:
	kubectl get pods,svc -n $(K8S_NAMESPACE) -l app=opsassistant

k8s-opsassistant-logs:
	kubectl logs -n $(K8S_NAMESPACE) deploy/opsassistant --tail=100


# ================== АГРЕГИРОВАННЫЕ ТАРГЕТЫ ==================

# Собрать и залить ВСЕ образы в kind
k8s-build-and-load-all: \
	users-build-and-load \
	orders-build-and-load \
	payments-build-and-load \
	outboxer-build-and-load \
	gateway-build-and-load \
	opsassistant-build-and-load

k8s-apply-all: \
	k8s-users-apply \
	k8s-orders-apply \
	k8s-payments-apply \
	k8s-outboxer-apply \
	k8s-gateway-apply \
	k8s-opsassistant-apply

# Полный bootstrap: кластер + образы + деплойменты + ingress
k8s-bootstrap: k8s-kind-create k8s-build-and-load-all k8s-apply-all k8s-ingress-bootstrap
	@echo "Кластер $(KIND_CLUSTER) полностью развернут (services + ingress)."

# Красивый статус всего кластера: наши сервисы + ingress-nginx
k8s-status-all:
	@echo "=== Namespace: $(K8S_NAMESPACE) - Pods (wide) ==="
	kubectl get pods -n $(K8S_NAMESPACE) -o wide || true
	@echo ""
	@echo "=== Namespace: $(K8S_NAMESPACE) - Services ==="
	kubectl get svc -n $(K8S_NAMESPACE) || true
	@echo ""
	@echo "=== Namespace: $(K8S_NAMESPACE) - Ingress ==="
	kubectl get ingress -n $(K8S_NAMESPACE) || true
	@echo ""
	@echo "=== Namespace: ingress-nginx - Pods ==="
	kubectl get pods -n ingress-nginx || true
	@echo ""
	@echo "=== Namespace: ingress-nginx - Services ==="
	kubectl get svc -n ingress-nginx || true


# ================== CLEAN: удалить все ресурсы в неймспейсе (кроме самого кластера) ==================

k8s-clean:
	@echo "=== Deleting all workload resources in namespace $(K8S_NAMESPACE) ==="
	-kubectl delete all -n $(K8S_NAMESPACE) --all
	@echo ""
	@echo "=== Deleting ingresses in namespace $(K8S_NAMESPACE) ==="
	-kubectl delete ingress -n $(K8S_NAMESPACE) --all
	@echo ""
	@echo "=== Deleting configmaps in namespace $(K8S_NAMESPACE) ==="
	-kubectl delete configmap -n $(K8S_NAMESPACE) --all
	@echo ""
	@echo "Namespace $(K8S_NAMESPACE) очищен (кластер и ingress-nginx остаются нетронутыми)."


# ================== REDEPLOY PER SERVICE + ALL ==================

k8s-redeploy-users: users-build-and-load
	$(MAKE) k8s-users-apply
	@echo "users redeployed to kind cluster $(KIND_CLUSTER) in namespace $(K8S_NAMESPACE)."

k8s-redeploy-orders: orders-build-and-load
	$(MAKE) k8s-orders-apply
	@echo "orders redeployed to kind cluster $(KIND_CLUSTER) in namespace $(K8S_NAMESPACE)."

k8s-redeploy-payments: payments-build-and-load
	$(MAKE) k8s-payments-apply
	@echo "payments redeployed to kind cluster $(KIND_CLUSTER) in namespace $(K8S_NAMESPACE)."

k8s-redeploy-outboxer: outboxer-build-and-load
	$(MAKE) k8s-outboxer-apply
	@echo "outboxer redeployed to kind cluster $(KIND_CLUSTER) in namespace $(K8S_NAMESPACE)."

k8s-redeploy-gateway: gateway-build-and-load
	$(MAKE) k8s-gateway-apply
	@echo "gateway redeployed to kind cluster $(KIND_CLUSTER) in namespace $(K8S_NAMESPACE)."

k8s-redeploy-opsassistant: opsassistant-build-and-load
	$(MAKE) k8s-opsassistant-apply
	@echo "opsassistant redeployed to kind cluster $(KIND_CLUSTER) in namespace $(K8S_NAMESPACE)."

k8s-redeploy-all: \
	users-build-and-load \
	orders-build-and-load \
	payments-build-and-load \
	outboxer-build-and-load \
	gateway-build-and-load \
	opsassistant-build-and-load
	$(MAKE) k8s-apply-all
	@echo "All services rebuilt, loaded into kind cluster $(KIND_CLUSTER) and reapplied in namespace $(K8S_NAMESPACE)."

docker-migrate-users:
	$(DOCKER_COMPOSE) -p $(DC_PROJECT) \
	  -f ./docker-compose.infra.yml \
	  -f ./docker-compose.users.yml \
	  up users-migrate --build

docker-migrate-orders:
	$(DOCKER_COMPOSE) -p $(DC_PROJECT) \
	  -f ./docker-compose.infra.yml \
	  -f ./docker-compose.orders.yml \
	  up orders-migrate --build

docker-migrate-payments:
	$(DOCKER_COMPOSE) -p $(DC_PROJECT) \
	  -f ./docker-compose.infra.yml \
	  -f ./docker-compose.payments.yml \
	  up payments-migrate --build

docker-migrate-all: docker-migrate-users docker-migrate-orders docker-migrate-payments
	@echo "All migrations applied to Postgres."
