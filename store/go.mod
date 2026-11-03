module cosmossdk.io/store

go 1.27.3

require (
	cosmossdk.io/errors v1.0.0
	cosmossdk.io/log v1.4.1
	cosmossdk.io/math v1.5.3
	github.com/cometbft/cometbft v1.0.1
	github.com/cometbft/cometbft/api v1.0.0
	github.com/cosmos/cosmos-db v1.1.3
	github.com/cosmos/gogoproto v1.7.2
	github.com/cosmos/iavl v1.2.8
	github.com/cosmos/ics23/go v0.11.0
	github.com/golang/mock v1.6.0
	github.com/hashicorp/go-hclog v1.6.3
	github.com/hashicorp/go-metrics v0.7.0
	github.com/hashicorp/go-plugin v1.8.0
	github.com/hashicorp/golang-lru v1.0.2
	github.com/stretchr/testify v1.12.1
	github.com/tidwall/btree v1.8.2
	golang.org/x/exp v0.0.0-20261014143641-f0251eb4aa3e
	google.golang.org/grpc v1.85.0
	google.golang.org/protobuf v1.36.12
	gotest.tools/v3 v3.5.2
)

require (
	github.com/DataDog/zstd v1.5.7 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cockroachdb/errors v1.14.0 // indirect
	github.com/cockroachdb/fifo v0.0.0-20240816210425-c5d0cb0b6fc0 // indirect
	github.com/cockroachdb/logtags v0.0.0-20241215232642-bb51bb14a506 // indirect
	github.com/cockroachdb/pebble v1.1.5 // indirect
	github.com/cockroachdb/redact v1.1.8 // indirect
	github.com/cockroachdb/tokenbucket v0.0.0-20250429170803-42689b6311bb // indirect
	github.com/emicklei/dot v1.11.0 // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/getsentry/sentry-go v0.50.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-uuid v1.0.4 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/jhump/protoreflect v1.19.0 // indirect
	github.com/klauspost/compress v1.20.3 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/linxGnu/grocksdb v1.11.1 // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oklog/run v1.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.25.0 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.1 // indirect
	github.com/prometheus/procfs v0.23.0 // indirect
	github.com/rogpeppe/go-internal v1.16.0 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261026165753-2ffab32bce7e // indirect
)

replace (
	github.com/cometbft/cometbft => github.com/InjectiveLabs/cometbft v1.0.1-inj.11
	github.com/cometbft/cometbft/api => github.com/InjectiveLabs/cometbft/api v1.0.0-inj.5
)

replace github.com/cosmos/iavl => github.com/InjectiveLabs/iavl v1.2.8-inj.1
