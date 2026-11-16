# ============================================================
# k8s-platform-lab : la "fiche recette" du projet
# Tape `make help` pour voir toutes les commandes.
# ============================================================

# Bash strict : une commande qui échoue arrête la recette
SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c
.DEFAULT_GOAL := help

# ---------- Versions des outils (mises à jour par Renovate) ----------
# Les commentaires "renovate:" permettent au bot de trouver et monter ces versions.
# Argo CD et metrics-server sont épinglés dans bootstrap/ (une seule source de vérité).
# renovate: datasource=github-releases depName=kubernetes-sigs/kind
KIND_VERSION ?= v0.33.0
# renovate: datasource=github-releases depName=kyverno/kyverno
KYVERNO_VERSION ?= v1.20.0
# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.15.0
# renovate: datasource=github-releases depName=aquasecurity/trivy
TRIVY_VERSION ?= v0.76.2
# renovate: datasource=github-releases depName=yannh/kubeconform
KUBECONFORM_VERSION ?= v0.7.0
# Version de Kubernetes simulée par envtest (leçon 05)
ENVTEST_K8S_VERSION ?= 1.37.0

# ---------- Variables du projet ----------
CLUSTER ?= platform-lab
TARGET  ?= app
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BIN     := $(CURDIR)/bin

# Les outils sont lancés à la version exacte, sans rien installer à la main
KIND          := go run sigs.k8s.io/kind@$(KIND_VERSION)
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
KUBECONFORM   := go run github.com/yannh/kubeconform/cmd/kubeconform@$(KUBECONFORM_VERSION)
# Trivy et le CLI Kyverno tournent en conteneur (TRIVY_VERSION:v%=% retire le "v")
TRIVY         := docker run --rm -v "$(CURDIR)":/src -w /src aquasec/trivy:$(TRIVY_VERSION:v%=%)
KYVERNO       := docker run --rm -v "$(CURDIR)":/work -w /work ghcr.io/kyverno/kyverno-cli:$(KYVERNO_VERSION)

# Dossiers kustomize à rendre (ceux qui n'existent pas encore sont ignorés)
KUSTOMIZE_DIRS := deploy/namespaces deploy/app/overlays/dev deploy/app/overlays/prod deploy/operator deploy/policies

.PHONY: help lint test manifests image render validate policy-test scan up down

##@ Aide

help: ## Affiche cette aide
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2} /^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5)}' $(MAKEFILE_LIST)

##@ Code Go

lint: ## Relit le code Go (golangci-lint, gosec inclus)
	$(GOLANGCI_LINT) run ./...

test: ## Lance les tests Go + couverture (+ envtest si l'opérateur existe)
	@if grep -q setup-envtest go.mod; then \
	  export KUBEBUILDER_ASSETS="$$(go tool setup-envtest use $(ENVTEST_K8S_VERSION) --bin-dir $(BIN)/envtest -p path)"; \
	fi; \
	go test ./... -coverprofile=cover.out

manifests: ## Régénère deepcopy + CRD + RBAC depuis les markers (leçon 05)
	@if grep -q controller-gen go.mod; then \
	  go tool controller-gen object:headerFile=hack/boilerplate.go.txt paths=./api/... && \
	  go tool controller-gen crd rbac:roleName=webapp-operator paths=./... \
	    output:crd:artifacts:config=deploy/operator/crd output:rbac:artifacts:config=deploy/operator/rbac; \
	else echo "pas encore d'opérateur (leçon 05)"; fi

image: ## Construit l'image locale (TARGET=app ou TARGET=operator)
	docker build --build-arg TARGET=$(TARGET) --build-arg VERSION=$(VERSION) \
	  --build-arg COMMIT=$(COMMIT) --build-arg DATE=$(DATE) -t $(TARGET):local .

##@ Manifests Kubernetes

render: ## Écrit le YAML final de chaque dossier kustomize dans _rendered/
	@mkdir -p _rendered
	@for d in $(KUSTOMIZE_DIRS); do \
	  if [ -f "$$d/kustomization.yaml" ]; then \
	    out="_rendered/$$(echo $$d | sed 's#/#-#g').yaml"; \
	    kubectl kustomize "$$d" > "$$out"; echo "rendu : $$out"; \
	  else echo "ignoré (pas encore créé) : $$d"; fi; \
	done

validate: render ## Valide le YAML rendu contre les schémas Kubernetes
	@if ls _rendered/*.yaml >/dev/null 2>&1; then \
	  $(KUBECONFORM) -strict -summary -ignore-missing-schemas _rendered/*.yaml; \
	else echo "rien à valider pour l'instant (leçon 04)"; fi

##@ Sécurité

policy-test: ## Teste les politiques Kyverno (leçon 07)
	@if [ -f deploy/policies/tests/kyverno-test.yaml ]; then \
	  $(KYVERNO) test deploy/policies/tests; \
	else echo "pas encore de policies (leçon 07)"; fi

scan: render ## Scan Trivy local : dépendances (fs) + mauvaises configs (config)
	$(TRIVY) fs --severity CRITICAL,HIGH --ignore-unfixed --exit-code 1 .
	$(TRIVY) config --exit-code 1 $(if $(wildcard .trivyignore.yaml),--ignorefile .trivyignore.yaml) _rendered

##@ Cluster local

up: ## Crée tout le lab : kind + metrics-server + Kyverno + Argo CD + app-of-apps
	$(KIND) create cluster --config bootstrap/kind-config.yaml
	kubectl apply -k bootstrap/metrics-server
	kubectl apply --server-side -f https://github.com/kyverno/kyverno/releases/download/$(KYVERNO_VERSION)/install.yaml
	kubectl -n kyverno rollout status deploy --timeout=300s
	kubectl create ns argocd --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply --server-side --force-conflicts -k bootstrap/argocd
	kubectl -n argocd rollout status deploy/argocd-server --timeout=300s
	kubectl apply -f gitops/root.yaml

down: ## Détruit le cluster kind
	$(KIND) delete cluster --name $(CLUSTER)
