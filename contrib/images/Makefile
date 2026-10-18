all: simd-env

simd-env: simd-rmi go-modules
	docker build --tag cosmossdk/simd -f simd-env/Dockerfile \
		$(shell git rev-parse --show-toplevel)

simd-dlv: simd-rmi go-modules
	docker build --tag cosmossdk/simd -f simd-dlv/Dockerfile \
		$(shell git rev-parse --show-toplevel)

# Stage private InjectiveLabs modules into .modcache for the Docker builds.
# Needs GH_TOKEN (or git credentials) with read access to them.
go-modules:
	$(shell git rev-parse --show-toplevel)/scripts/prepare-docker-go-modules.sh

simd-rmi:
	docker rmi cosmossdk/simd 2>/dev/null; true

.PHONY: all simd-env simd-dlv simd-rmi go-modules
