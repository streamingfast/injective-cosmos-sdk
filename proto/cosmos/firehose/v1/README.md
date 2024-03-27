# Generate

```shell
buf generate --template ./buf.gen.gogo.firehose.yaml cosmos/firehose/v1/block.proto
cd ../store/streaming/firehose
mv cosmos/firehose/v1/block.pb.go .
rm -rf cosmos/
```