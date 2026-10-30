
,perftools/tracing/proto/rpc_trace_info.protoperftools.tracingnet/base/qos.proto)net/proto2/bridge/proto/message_set.proto"Ô
RPCFlags"‚
Flags
REQUEST_COMPRESSION_ZIPPY
DUPLICATE_SUPPRESSION
	STREAMING
	FAIL_FAST
RESPONSE_COMPRESSION_ZIPPY
REQUEST_COMPRESSION_GIPFELI  
RESPONSE_COMPRESSION_GIPFELI@
BIDIRECTIONAL_STREAMINGÄ
REQUEST_COMPRESSION_BROTLIÄ 
RESPONSE_COMPRESSION_BROTLIÄ
REQUEST_COMPRESSION_ZSTDÄ
RESPONSE_COMPRESSION_ZSTDÄ"˜
RPCClientRequestInfo
size_bytes_wire (
size_bytes_app (
protocol
 (:0
security_level (
	server_ip (
server_ip46 (
server_port (
client_port (
enqueue_time (
deadline (
flags	 (:1
messaged (	2d
message_set_extension.proto2.bridge.MessageSet›»ı (2'.perftools.tracing.RPCClientRequestInfo"^
RPCServerProcessingInfo
server_start_time (
server_elapsed_time (:R	OPAQUE_V0"«
RPCClientResponseInfo
size_bytes_wire (
size_bytes_app (
status (:0
security_level (?
server_info (2*.perftools.tracing.RPCServerProcessingInfo
client_receive_time (
num_responses (6
rcvd_traffic_class (2.net_base.qos.TrafficClass
rcvd_b4_path_selector	 (

query_cost
 (2e
message_set_extension.proto2.bridge.MessageSetıÖˆ (2(.perftools.tracing.RPCClientResponseInfo:R	OPAQUE_V0"√
RPCServerRequestInfo
size_bytes_wire (
size_bytes_app (
protocol (:0
security_level (
	client_ip (
client_ip46 (
client_port (
server_port	 (
deadline (
server_receive_time (6
rcvd_traffic_class
 (2.net_base.qos.TrafficClass
rcvd_b4_path_selector (
messaged (	2d
message_set_extension.proto2.bridge.MessageSetÍöˆ (2'.perftools.tracing.RPCServerRequestInfo"ÿ
RPCServerResponseInfo
size_bytes_wire (
size_bytes_app (
enqueue_time (C
processing_info (2*.perftools.tracing.RPCServerProcessingInfo
flags (:1
num_responses (

query_cost (
messaged (	2e
message_set_extension.proto2.bridge.MessageSet—â˜ (2(.perftools.tracing.RPCServerResponseInfo"—
RPCStreamingResponseInfo
size_bytes_app (
size_bytes_wire (
seqno (2h
message_set_extension.proto2.bridge.MessageSet∑Å† (2+.perftools.tracing.RPCStreamingResponseInfo:R	OPAQUE_V0B0
com.google.tracing(ÚOPEN_TO_OPAQUE_HYBRID