module github.com/m-lab/locate

go 1.25.5

require (
	cloud.google.com/go/compute/metadata v0.9.0
	cloud.google.com/go/secretmanager v1.16.0
	github.com/alicebob/miniredis v2.5.0+incompatible
	github.com/apex/log v1.9.0
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/go-jose/go-jose/v4 v4.1.3
	github.com/go-test/deep v1.1.1
	github.com/gomodule/redigo v1.9.3
	github.com/google/uuid v1.6.0
	github.com/googleapis/gax-go/v2 v2.16.0
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674
	github.com/hashicorp/go-multierror v1.1.1
	github.com/justinas/alice v1.2.0
	github.com/m-lab/access v0.0.13
	github.com/m-lab/go v0.1.76
	github.com/m-lab/uuid-annotator v0.5.10
	github.com/oschwald/geoip2-golang v1.13.0
	github.com/paulmach/orb v0.12.0
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/client_model v0.6.2
	github.com/prometheus/common v0.67.5
	github.com/rafaeljusto/redigomock v2.4.0+incompatible
	github.com/sirupsen/logrus v1.9.4
	golang.org/x/net v0.49.0
	google.golang.org/api v0.264.0
	google.golang.org/genproto v0.0.0-20251222181119-0a764e51fe1b
	k8s.io/api v0.35.0
	k8s.io/apimachinery v0.35.0
	k8s.io/client-go v0.35.0
)

require (
	cel.dev/expr v0.25.1 // indirect
	cloud.google.com/go/auth v0.18.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/monitoring v1.24.3 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.30.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.54.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.54.0 // indirect
	github.com/alicebob/gopher-json v0.0.0-20230218143504-906a9b012302 // indirect
	github.com/cncf/xds/go v0.0.0-20251210132809-ee656c7534f5 // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.36.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.22.4 // indirect
	github.com/go-openapi/jsonreference v0.21.4 // indirect
	github.com/go-openapi/swag v0.25.4 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.4 // indirect
	github.com/go-openapi/swag/conv v0.25.4 // indirect
	github.com/go-openapi/swag/fileutils v0.25.4 // indirect
	github.com/go-openapi/swag/jsonname v0.25.4 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.4 // indirect
	github.com/go-openapi/swag/loading v0.25.4 // indirect
	github.com/go-openapi/swag/mangling v0.25.4 // indirect
	github.com/go-openapi/swag/netutils v0.25.4 // indirect
	github.com/go-openapi/swag/stringutils v0.25.4 // indirect
	github.com/go-openapi/swag/typeutils v0.25.4 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.4 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/google/gnostic-models v0.7.1 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.11 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spiffe/go-spiffe/v2 v2.6.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.mongodb.org/mongo-driver v1.17.6 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.39.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.64.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.64.0 // indirect
	go.opentelemetry.io/otel v1.39.0 // indirect
	go.opentelemetry.io/otel/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/sdk v1.39.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/trace v1.39.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sync v0.19.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260122232226-8e98ce8d340d // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	k8s.io/kube-openapi v0.0.0-20251125145642-4e65d59e963e // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.1 // indirect
)

require (
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/compute v1.54.0
	cloud.google.com/go/iam v1.5.3 // indirect
	cloud.google.com/go/storage v1.58.0 // indirect
	github.com/aptible/supercronic v0.2.41
	github.com/araddon/dateparse v0.0.0-20210429162001-6b43995a97de // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/google/go-cmp v0.7.0
	github.com/googleapis/google-cloud-go-testing v0.0.0-20210719221736-1c9a4c676720 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/oschwald/maxminddb-golang v1.13.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	golang.org/x/crypto v0.47.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/term v0.39.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20260106112306-0fe9cd71b2f8 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)
