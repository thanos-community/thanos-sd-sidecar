mkdir -p example/prometheus0_eu1_data example/prometheus0_us1_data example/prometheus1_us1_data

DOCKER=$(which docker)

# NOTE: This setup uses host.docker.internal as the hostname for allowing containers to access services outside as net=host isn't consistent.
# Docker verion 20.10.0+ is needed for this to work on Linux along with the `--add-host=host.docker.internal:host-gateway` flag while running containers.

# Start 3 Prometheus instances, one in EU1 and two replicas in US1.

# EU1
$DOCKER run -d -p 9090:9090 \
    --add-host=host.docker.internal:host-gateway \
    -v $(pwd)/example/:/shared \
    --name prometheus-0-eu1 \
    quay.io/prometheus/prometheus:latest \
    --config.file=/shared/prometheus0_eu1.yaml \
    --storage.tsdb.path=/shared/prometheus0_eu1_data \
    --web.listen-address=:9090 \
    --web.enable-lifecycle \
    --web.enable-admin-api

sleep 1

# US1 Replica 0
$DOCKER run -d -p 9091:9091 \
    --add-host=host.docker.internal:host-gateway \
    -v $(pwd)/example/:/shared \
    --name prometheus-0-us1 \
    quay.io/prometheus/prometheus:latest \
    --config.file=/shared/prometheus0_us1.yaml \
    --storage.tsdb.path=/shared/prometheus0_us1_data \
    --web.listen-address=:9091 \
    --web.enable-lifecycle \
    --web.enable-admin-api

sleep 1

# US1 Replica 1
$DOCKER run -d -p 9092:9092 \
    --add-host=host.docker.internal:host-gateway \
    -v $(pwd)/example/:/shared \
    --name prometheus-1-us1 \
    quay.io/prometheus/prometheus:latest \
    --config.file=/shared/prometheus1_us1.yaml \
    --storage.tsdb.path=/shared/prometheus1_us1_data \
    --web.listen-address=:9092 \
    --web.enable-lifecycle \
    --web.enable-admin-api

sleep 3

# Start Thanos Sidecars for all of them.

$DOCKER run -d \
    --add-host=host.docker.internal:host-gateway \
    -p 19090:19090 \
    -p 19190:19190 \
    -v $(pwd)/example/:/shared \
    --name thanos-sidecar-0-eu1 \
    quay.io/thanos/thanos:v0.23.1 sidecar \
    --http-address 0.0.0.0:19090 \
    --grpc-address 0.0.0.0:19190 \
    --reloader.config-file=/shared/prometheus0_eu1.yaml \
    --prometheus.url=http://host.docker.internal:9090

sleep 1

$DOCKER run -d \
    --add-host=host.docker.internal:host-gateway \
    -p 19091:19091 \
    -p 19191:19191 \
    -v $(pwd)/example/:/shared \
    --name thanos-sidecar-0-us1 \
    quay.io/thanos/thanos:v0.23.1 sidecar \
    --http-address 0.0.0.0:19091 \
    --grpc-address 0.0.0.0:19191 \
    --reloader.config-file=/shared/prometheus0_us1.yaml \
    --prometheus.url=http://host.docker.internal:9091

sleep 1

$DOCKER run -d \
    --add-host=host.docker.internal:host-gateway \
    -p 19092:19092 \
    -p 19192:19192 \
    -v $(pwd)/example/:/shared \
    --name thanos-sidecar-1-us1 \
    quay.io/thanos/thanos:v0.23.1 sidecar \
    --http-address 0.0.0.0:19092 \
    --grpc-address 0.0.0.0:19192 \
    --reloader.config-file=/shared/prometheus1_us1.yaml \
    --prometheus.url=http://host.docker.internal:9092

sleep 1

# Start Consul agent with service registration for above 6 containers.

$DOCKER run \
    -d \
    --add-host=host.docker.internal:host-gateway \
    -v $(pwd)/example/:/shared \
    -p 8500:8500 \
    -p 8600:8600/udp \
    --name=consul-agent \
    consul agent \
    --enable-script-checks \
    --config-dir=/shared/consul.d \
    -server -ui -node=server-1 -bootstrap-expect=1 -client=0.0.0.0

sleep 3

# Run Thanos SD Sidecar which is configured to query Consul for services and filter them if they are Thanos stores.

$DOCKER run -d \
    --add-host=host.docker.internal:host-gateway \
    -p 8000:8000 \
    -v $(pwd)/example/:/shared \
    --name=thanos-sd-sidecar \
    thanos-sd-sidecar run \
    --http.sd \
    --config-file=/shared/config.yaml \
    --output.path=/shared/targets.json

sleep 3

# Finally, run Thanos Querier to see them in action.

$DOCKER run -d \
    --add-host=host.docker.internal:host-gateway \
    -p 29090:29090 \
    -v $(pwd)/example/:/shared \
    --name thanos-query \
    quay.io/thanos/thanos:v0.23.1 query \
    --http-address 0.0.0.0:29090 \
    --query.replica-label replica \
    --store.sd-files=/shared/targets.json

sleep 6

# open http://localhost:29090/stores
