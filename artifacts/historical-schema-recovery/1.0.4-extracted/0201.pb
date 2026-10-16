
1third_party/jetski/api_server_pb/api_server.protoexa.api_server_pb3google/internal/cloud/code/v1internal/credits.protogoogle/protobuf/timestamp.proto/third_party/gemini_coder/proto/trajectory.proto/third_party/jetski/analytics_pb/analytics.proto%third_party/jetski/chat_pb/chat.proto9third_party/jetski/codeium_common_pb/codeium_common.proto9third_party/jetski/context_module_pb/context_module.proto)third_party/jetski/cortex_pb/cortex.proto7third_party/jetski/jetski_cortex_pb/jetski_cortex.proto)third_party/jetski/prompt_pb/prompt.proto"¶
StreamingTestRequest
length (Rlength*
sleep_duration_ms (RsleepDurationMs

num_chunks (R	numChunks+
randomize_content (RrandomizeContent"3
StreamingTestResponse
response (	Rresponse"U
GetCascadeNuxesRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata"X
GetCascadeNuxesResponse=
nuxes (2'.exa.codeium_common_pb.CascadeNUXConfigRnuxes"≠
GetTranscriptionRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata

audio_data (R	audioData
prompt (	Rprompt^
latency_info (2;.exa.api_server_pb.GetTranscriptionRequest.LatencyInfoEntryRlatencyInfo>
LatencyInfoEntry
key (	Rkey
value (Rvalue:8"E
GetTranscriptionResponse)
transcribed_text (	RtranscribedText"÷
GetVideoSummarizationRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
video_bytes (R
videoBytes
	mime_type (	RmimeType;

chat_model (2.exa.codeium_common_pb.ModelR	chatModel"9
GetVideoSummarizationResponse
summary (	Rsummary"í
GetImageGenerationRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
prompt (	Rprompt2
model (2.exa.codeium_common_pb.ModelRmodel`
latency_info (2=.exa.api_server_pb.GetImageGenerationRequest.LatencyInfoEntryRlatencyInfoG
input_images (2 .exa.codeium_common_pb.ImageDataBRinputImages=
input_media (2.exa.codeium_common_pb.MediaR
inputMedia#
trajectory_id (	RtrajectoryId

step_index (R	stepIndex>
LatencyInfoEntry
key (	Rkey
value (Rvalue:8"æ
GetImageGenerationResponse:
image (2 .exa.codeium_common_pb.ImageDataBRimage

model_name (	R	modelNameE
generated_media (2.exa.codeium_common_pb.MediaRgeneratedMedia"¨
RecordNewCortexPlanRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
	cortex_id (	RcortexIdI
request_source (2".exa.cortex_pb.CortexRequestSourceRrequestSource
	plan_goal (	RplanGoal
input_files (	R
inputFiles+
input_directories (	RinputDirectories"
RecordNewCortexPlanResponse"™
RecordCortexErrorRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
	cortex_id (	RcortexId
error (	Rerror!
error_detail (	RerrorDetailF
category (2*.exa.codeium_common_pb.CortexErrorCategoryRcategory
category_id (	R
categoryId
test (	Rtest"
RecordCortexErrorResponse"·
RecordCortexCodingPlanRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata$
coding_plan_id (	RcodingPlanId
	cortex_id (	RcortexId(
coding_plan_goal (	RcodingPlanGoalH
model_config (2%.exa.codeium_common_pb.ChatNodeConfigRmodelConfig#
system_prompt (	RsystemPromptG
message_prompts (2.exa.chat_pb.ChatMessagePromptRmessagePrompts*
num_output_tokens
 (RnumOutputTokens2
response_latencies_ms (RresponseLatenciesMs" 
RecordCortexCodingPlanResponse"÷
RecordCortexCodingStepRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata$
coding_step_id (	RcodingStepId$
coding_plan_id (	RcodingPlanId

step_index (R	stepIndex
action_type (	R
actionType 
instruction (	Rinstruction
source (	Rsource
target (	Rtarget(
action_spec_json	 (	RactionSpecJson*
step_outline_json
 (	RstepOutlineJson!
step_version (RstepVersion!
plan_version (RplanVersion" 
RecordCortexCodingStepResponse"Ï
%RecordCortexCodingStepFeedbackRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata$
coding_step_id (	RcodingStepId;
feedback (2.exa.api_server_pb.FeedbackTypeRfeedback#
completion_id (	RcompletionId"(
&RecordCortexCodingStepFeedbackResponse"´
RecordCortexFeedbackRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
	cortex_id (	RcortexId
feedback (	Rfeedback
rating (Rrating"
RecordCortexFeedbackResponse"Ê
RecordDebounceRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadataN
provider_source (2%.exa.codeium_common_pb.ProviderSourceRproviderSource;
language (2.exa.codeium_common_pb.LanguageRlanguage'
editor_language (	ReditorLanguage*
prompt_latency_ms (RpromptLatencyMs.
debounce_latency_ms (RdebounceLatencyMs"
RecordDebounceResponse"¯
OpportunityMetadata;
metadata (2.exa.codeium_common_pb.MetadataRmetadata;
language (2.exa.codeium_common_pb.LanguageRlanguageN
completion_type (2%.exa.codeium_common_pb.CompletionTypeRcompletionTypeT
completion_source (2'.exa.codeium_common_pb.CompletionSourceRcompletionSourceA

repository (2!.exa.codeium_common_pb.RepositoryR
repository"j
RecordOpportunitiesRequestL
opportunities (2&.exa.api_server_pb.OpportunityMetadataRopportunities"
RecordOpportunitiesResponse"π
ByteDeltaInfoB
code_source (2!.exa.codeium_common_pb.CodeSourceR
codeSource
	num_bytes (RnumBytesN
completion_type (2%.exa.codeium_common_pb.CompletionTypeRcompletionTypeN
provider_source (2%.exa.codeium_common_pb.ProviderSourceRproviderSource'
generator_model (	RgeneratorModel"–
CodeTrackerUpdate;
metadata (2.exa.codeium_common_pb.MetadataRmetadata;
language (2.exa.codeium_common_pb.LanguageRlanguageA
byte_deltas (2 .exa.api_server_pb.ByteDeltaInfoR
byteDeltas"a
RecordCodeTrackerUpdatesRequest>
updates (2$.exa.api_server_pb.CodeTrackerUpdateRupdates""
 RecordCodeTrackerUpdatesResponse"d
RecordCompletionExampleRequestB
example (2(.exa.codeium_common_pb.CompletionExampleRexample"!
RecordCompletionExampleResponse"l
CciWithSubrangeBatchT
cci_with_subranges (2&.exa.codeium_common_pb.CciWithSubrangeRcciWithSubranges"¡
EarlyReturnConfig2
min_complete_fraction (RminCompleteFraction3
min_additional_wait_ms (RminAdditionalWaitMsC
min_additional_wait_multiplier (RminAdditionalWaitMultiplier"”
GetMQueryRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata2
model (2.exa.codeium_common_pb.ModelRmodel
query (	Rqueryb
cci_with_subrange_batches (2'.exa.api_server_pb.CciWithSubrangeBatchRcciWithSubrangeBatchesT
early_return_config (2$.exa.api_server_pb.EarlyReturnConfigRearlyReturnConfig"{
ScoreWithStatus<
status (2$.exa.api_server_pb.MQueryScoreStatusRstatus
score (Rscore
error (	Rerror"H

ScoreBatch:
scores (2".exa.api_server_pb.ScoreWithStatusRscores"£
GetMQueryResponseB
score_batches (2.exa.api_server_pb.ScoreBatchRscoreBatches
scorer_name (	R
scorerName)
tokens_processed (RtokensProcessed"Ÿ
*GetStreamingExternalChatCompletionsRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadataP
chat_message_prompts (2.exa.chat_pb.ChatMessagePromptRchatMessagePrompts#
system_prompt (	RsystemPromptZ
completions_request (2).exa.codeium_common_pb.CompletionsRequestRcompletionsRequestN
provider_source (2%.exa.codeium_common_pb.ProviderSourceRproviderSource7
model_id (2.exa.codeium_common_pb.ModelRmodelIdT
experiment_config (2'.exa.codeium_common_pb.ExperimentConfigRexperimentConfig
	prompt_id	 (	RpromptIdJRskip_completion_telemetry"è
+GetStreamingExternalChatCompletionsResponseF
	delta_map (2).exa.codeium_common_pb.CompletionDeltaMapRdeltaMap
latency (Rlatency"Ç
GetEmbeddingsRequestB
request (2(.exa.codeium_common_pb.EmbeddingsRequestRrequestJ
retry_policy (2'.exa.api_server_pb.EmbeddingRetryPolicyRretryPolicyE
embedding_model (2.exa.codeium_common_pb.ModelRembeddingModelJRhigh_priority"w
GetEmbeddingsResponseD
response (2(.exa.codeium_common_pb.EmbeddingResponseRresponse
latency (Rlatency"°
%BatchRecordUserLastUpdateTimesRequestÜ
user_last_update_times (2Q.exa.api_server_pb.BatchRecordUserLastUpdateTimesRequest.UserLastUpdateTimesEntryRuserLastUpdateTimeso
UserLastUpdateTimesEntry
key (	Rkey=
value (2'.exa.codeium_common_pb.LastUpdateRecordRvalue:8"(
&BatchRecordUserLastUpdateTimesResponse"¨
RecordChatRequestRecordRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadataK
chat_request (2(.exa.api_server_pb.GetChatMessageRequestRchatRequest+
disable_telemetry (RdisableTelemetry

deployment (	R
deployment

start_time (	R	startTime
end_time (	RendTimeB
user_status (2!.exa.codeium_common_pb.UserStatusR
userStatus
error (	Rerror
request_uid	 (	R
requestUid"|
$BatchRecordChatRequestRecordsRequestT
chat_records (21.exa.api_server_pb.RecordChatRequestRecordRequestRchatRecords"'
%BatchRecordChatRequestRecordsResponse"Z
GetCompletionExamplesRequest!
max_examples (RmaxExamples
api_key (	RapiKey"q
GetCompletionExamplesResponseP
examples (24.exa.codeium_common_pb.CompletionExampleWithMetadataRexamples"ê
ProvideFeedbackRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
	prompt_id (	RpromptId#
completion_id (	RcompletionId?
source (2'.exa.codeium_common_pb.CompletionSourceRsource;
feedback (2.exa.api_server_pb.FeedbackTypeRfeedbackC

extra_info (2$.exa.api_server_pb.FeedbackExtraInfoR	extraInfo

latency_ms (R	latencyMs,
network_latency_ms (RnetworkLatencyMs.
debounce_latency_ms	 (RdebounceLatencyMs*
client_latency_ms
 (RclientLatencyMs*
timestamp_unix_ms (RtimestampUnixMs*
feedback_delay_ms (RfeedbackDelayMsN
provider_source (2%.exa.codeium_common_pb.ProviderSourceRproviderSource*
view_columns_open (RviewColumnsOpen7
has_active_vim_extension (RhasActiveVimExtension"
ProvideFeedbackResponse"ù
UploadErrorTracesRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadataD
error_traces (2!.exa.codeium_common_pb.ErrorTraceRerrorTraces"
UploadErrorTracesResponse"∫
RecordAsyncTelemetryRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadataD
error_traces (2!.exa.codeium_common_pb.ErrorTraceRerrorTraces[
workspace_index_data (2).exa.codeium_common_pb.WorkspaceIndexDataRworkspaceIndexDataN
workspace_stats (2%.exa.codeium_common_pb.WorkspaceStatsRworkspaceStats!
memory_stats (RmemoryStatsH
indexer_stats (2#.exa.codeium_common_pb.IndexerStatsRindexerStats"
RecordAsyncTelemetryResponse"Ê
GetChatMessageRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
prompt (	RpromptP
chat_message_prompts (2.exa.chat_pb.ChatMessagePromptRchatMessagePrompts5
use_internal_chat_model (RuseInternalChatModelL
internal_chat_model (2.exa.codeium_common_pb.ModelRinternalChatModel]
custom_model_info_override (2 .exa.codeium_common_pb.ModelInfoRcustomModelInfoOverrideL
request_type (2).exa.api_server_pb.ChatMessageRequestTypeRrequestTypeT
configuration (2..exa.codeium_common_pb.CompletionConfigurationRconfigurationT
experiment_config	 (2'.exa.codeium_common_pb.ExperimentConfigRexperimentConfig5
tools
 (2.exa.chat_pb.ChatToolDefinitionRtools<
tool_choice (2.exa.chat_pb.ChatToolChoiceR
toolChoice^
system_prompt_cache_options (2.exa.chat_pb.PromptCacheOptionsRsystemPromptCacheOptions&
chat_model_name (	RchatModelName[
trajectory_reference (2(.exa.cortex_pb.CortexTrajectoryReferenceRtrajectoryReference
	prompt_id (	RpromptIdN
provider_source (2%.exa.codeium_common_pb.ProviderSourceRproviderSource;
language (2.exa.codeium_common_pb.LanguageRlanguage*
enable_ai_credits (RenableAiCredits'
conversation_id (	RconversationIde
custom_metadata (2<.exa.api_server_pb.GetChatMessageRequest.CustomMetadataEntryRcustomMetadataA
CustomMetadataEntry
key (	Rkey
value (	Rvalue:8JJJR
model_nameRdisable_parallel_tool_callsR
cascade_id"´
GetChatMessageResponse

message_id (	R	messageId

delta_text (	R	deltaText!
delta_tokens (RdeltaTokensB
stop_reason (2!.exa.codeium_common_pb.StopReasonR
stopReasonM
delta_tool_calls (2#.exa.codeium_common_pb.ChatToolCallRdeltaToolCalls<
usage (2&.exa.codeium_common_pb.ModelUsageStatsRusage0
delta_raw_generation (	RdeltaRawGeneration%
delta_thinking	 (	RdeltaThinking,
delta_raw_thinking (	RdeltaRawThinking'
delta_signature
 (RdeltaSignature+
thinking_redacted (RthinkingRedacted
latency (RlatencyT
citation_metadata (2'.exa.codeium_common_pb.CitationMetadataRcitationMetadata!
sherlog_link (	RsherlogLink
trace_id (	RtraceId!
stop_message (	RstopMessageY
consumed_credits (2..google.internal.cloud.code.v1internal.CreditsRconsumedCredits%
response_model (	RresponseModel?
provider_assigned_message_id (	RproviderAssignedMessageIdJJ	JJJJR	timestampRredactRcompletion_profileRcredit_costR	output_idRthinking_id"˛
)GetStreamingModelAPITextCompletionRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata2
model (2.exa.codeium_common_pb.ModelRmodel#
system_prompt (	RsystemPrompt^
system_prompt_cache_options (2.exa.chat_pb.PromptCacheOptionsRsystemPromptCacheOptionsP
chat_message_prompts (2.exa.chat_pb.ChatMessagePromptRchatMessagePromptsi
completion_configuration (2..exa.codeium_common_pb.CompletionConfigurationRcompletionConfigurationL
request_type (2).exa.api_server_pb.ChatMessageRequestTypeRrequestTypeP
override_model_info (2 .exa.codeium_common_pb.ModelInfoRoverrideModelInfo"t
*GetStreamingModelAPITextCompletionResponseF
	delta_map (2).exa.codeium_common_pb.CompletionDeltaMapRdeltaMap"ì
RecordChatRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata

latency_ms (R	latencyMs;
chat_message (2.exa.chat_pb.ChatMessageRchatMessageP
chat_message_prompts (2.exa.chat_pb.ChatMessagePromptRchatMessagePrompts#
system_prompt (	RsystemPrompta
context_inclusion_type (2+.exa.codeium_common_pb.ContextInclusionTypeRcontextInclusionType8
experiment_features_json (	RexperimentFeaturesJson6
experiment_variant_json	 (	RexperimentVariantJson7
model_id (2.exa.codeium_common_pb.ModelRmodelId!
active_repos
 (	RactiveRepos)
guideline_prompt (	RguidelinePrompt\
chat_prompt_components (2&.exa.prompt_pb.UnifiedPromptComponentsRchatPromptComponentsh
cortex_trajectory_reference (2(.exa.cortex_pb.CortexTrajectoryReferenceRcortexTrajectoryReferenceJ
pinned_context (2#.exa.codeium_common_pb.ContextScopeRpinnedContext"
RecordChatResponse"Ñ
RecordChatFeedbackRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata

message_id (	R	messageId9
feedback (2.exa.chat_pb.ChatFeedbackTypeRfeedback
reason (	Rreason8
	timestamp (2.google.protobuf.TimestampR	timestamp"
RecordChatFeedbackResponse"‚
RecordChatPanelSessionRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadataC
start_timestamp (2.google.protobuf.TimestampRstartTimestamp?
end_timestamp (2.google.protobuf.TimestampRendTimestamp" 
RecordChatPanelSessionResponse"é
RecordContextRefreshRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadataR
refresh_reason (2+.exa.context_module_pb.ContextRefreshReasonRrefreshReason
did_refresh (R
didRefreshC
current_document_absolute_path (	RcurrentDocumentAbsolutePathC
current_document_relative_path (	RcurrentDocumentRelativePath<
current_document_workspace (	RcurrentDocumentWorkspace5
current_chat_message_id (	RcurrentChatMessageId[
current_document_language (2.exa.codeium_common_pb.LanguageRcurrentDocumentLanguage

latency_ms	 (R	latencyMs5
state_update_latency_ms
 (RstateUpdateLatencyMsu
code_context_counts (2E.exa.api_server_pb.RecordContextRefreshRequest.CodeContextCountsEntryRcodeContextCounts8
	timestamp (2.google.protobuf.TimestampR	timestamp4
local_codebase_queries (	RlocalCodebaseQueries9
context_module_stats_json (	RcontextModuleStatsJson#
refresh_error (	RrefreshErrorD
CodeContextCountsEntry
key (	Rkey
value (Rvalue:8"
RecordContextRefreshResponse"á
RecordEventRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata4
events (2.exa.codeium_common_pb.EventRevents"
RecordEventResponse"‡
RecordPinnedContextRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata9
scope (2#.exa.codeium_common_pb.ContextScopeRscopeJ
previous_scope (2#.exa.codeium_common_pb.ContextScopeRpreviousScope"
RecordPinnedContextResponse"Ó
RecordSearchRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
query (	Rquery
	search_id (	RsearchId

latency_ms (R	latencyMsH
search_type (2'.exa.codeium_common_pb.SearchResultTypeR
searchType"µ
RecordMQueryRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata%
refresh_reason (	RrefreshReason
query (	Rquery8
	timestamp (2.google.protobuf.TimestampR	timestamp#
items_scanned (RitemsScanned)
tokens_processed (RtokensProcessed&
job_duration_ms (RjobDurationMs

request_id (	R	requestId

error_json	 (	R	errorJson4
relevance_scorer_model
 (	RrelevanceScorerModel"
RecordMQueryResponse"ç
RecordChatModelNodeRunRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
run_id (	RrunId

mixin_type (	R	mixinType

latency_ms (R	latencyMs#
system_prompt (	RsystemPromptP
chat_message_prompts (2.exa.chat_pb.ChatMessagePromptRchatMessagePromptsa
context_inclusion_type (2+.exa.codeium_common_pb.ContextInclusionTypeRcontextInclusionType7
model_id (2.exa.codeium_common_pb.ModelRmodelId8
	timestamp	 (2.google.protobuf.TimestampR	timestamp
error
 (	Rerror%
num_iterations (RnumIterations+
completion_tokens (RcompletionTokens#
prompt_tokens (RpromptTokens" 
RecordChatModelNodeRunResponse"¿
RecordCortexStepRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata$
cortex_plan_id (	RcortexPlanId1

step_proto (2.gemini_coder.StepR	stepProto!
execution_id (	RexecutionIdI
state_snapshot (2".exa.cortex_pb.CortexWorkflowStateRstateSnapshot!
event_source (	ReventSource"
RecordCortexStepResponse"û
'RecordTrajectorySegmentAnalyticsRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadataG
trajectory_segment (2.gemini_coder.TrajectoryRtrajectorySegment(
start_step_index (RstartStepIndexC
start_generator_metadata_index (RstartGeneratorMetadataIndex"*
(RecordTrajectorySegmentAnalyticsResponse"ù
$RecordFullTrajectoryAnalyticsRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata8

trajectory (2.gemini_coder.TrajectoryR
trajectory"'
%RecordFullTrajectoryAnalyticsResponse"ä
$RecordTrajectorySegmentEventsRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata#
trajectory_id (	RtrajectoryId!
execution_id (	RexecutionId>
records (2$.exa.codeium_common_pb.MetricsRecordRrecords

event_type (	R	eventType"'
%RecordTrajectorySegmentEventsResponse"Ü
$RecordCortexGeneratorMetadataRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata#
trajectory_id (	RtrajectoryId

cascade_id (	R	cascadeIdY
generator_metadata (2*.exa.cortex_pb.CortexStepGeneratorMetadataRgeneratorMetadata%
metadata_index (RmetadataIndex*
last_common_index (RlastCommonIndex/
upload_to_full_table (RuploadToFullTable"'
%RecordCortexGeneratorMetadataResponse"ë
$RecordCommitMessageGenerationRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata4
generation_duration_ms (RgenerationDurationMs#
system_prompt (	RsystemPrompt
full_prompt (	R
fullPrompt#
prompt_tokens (RpromptTokens+
completion_tokens (RcompletionTokens
	repo_root (	RrepoRoot
branch_name (	R
branchName,
latest_commit_hash	 (	RlatestCommitHash*
num_files_changed
 (RnumFilesChanged&
num_lines_added (RnumLinesAdded*
num_lines_deleted (RnumLinesDeleted4
commit_message_summary (	RcommitMessageSummary<
commit_message_description (	RcommitMessageDescription"'
%RecordCortexExecutionMetadataResponse"Ú
$RecordCortexExecutionMetadataRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata!
execution_id (	RexecutionId

cascade_id (	R	cascadeId#
trajectory_id (	RtrajectoryId'
experiment_json (	RexperimentJson8
experiment_variants_json (	RexperimentVariantsJsonC
cascade_config (2.exa.cortex_pb.CascadeConfigRcascadeConfig"'
%RecordCommitMessageGenerationResponse"«
$RecordStateInitializationDataRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadatab
state_initialization_data (2&.exa.cortex_pb.StateInitializationDataRstateInitializationData"'
%RecordStateInitializationDataResponse"ú
RecordCommitMessageSaveRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
	repo_root (	RrepoRoot
branch_name (	R
branchName
commit_hash (	R
commitHash%
commit_message (	RcommitMessageE
commit_timestamp (2.google.protobuf.TimestampRcommitTimestamp,
parent_commit_hash (	RparentCommitHash
author_name (	R
authorName!
author_email	 (	RauthorEmail"!
RecordCommitMessageSaveResponse"
RecordSearchResponse"´
RecordSearchResultsRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadataP
search_results (2).exa.codeium_common_pb.SearchResultRecordRsearchResults"
RecordSearchResultsResponse"v
RecordSearchDocOpenRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
	result_id (	RresultId"
RecordSearchDocOpenResponse"›
RecordSearchResultsViewRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata*
cluster_search_id (	RclusterSearchId&
exact_search_id (	RexactSearchId*
search_result_ids (	RsearchResultIds"!
RecordSearchResultsViewResponse"
SupportsRemoteIndexingRequest"Z
SupportsRemoteIndexingResponse8
supports_remote_indexing (RsupportsRemoteIndexing"V
GetModelStatusesRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata"p
GetModelStatusesResponseT
model_status_infos (2&.exa.codeium_common_pb.ModelStatusInfoRmodelStatusInfos"
GetModelInfosRequest"Z
GetModelInfosResponseA
model_infos (2 .exa.codeium_common_pb.ModelInfoR
modelInfos"Ñ
SubscribeRequest*
firebase_id_token (	RfirebaseIdToken

newsletter (R
newsletter-
disabled_telemetry (RdisabledTelemetry%
inbound_source (	RinboundSource!
signup_stage (	RsignupStage+
registration_code (	RregistrationCode"
SubscribeResponse"E
GetSubscriptionsRequest*
firebase_id_token (	RfirebaseIdToken"À
GetSubscriptionsResponse
exists (Rexists

newsletter (R
newsletter-
disabled_telemetry (RdisabledTelemetry%
inbound_source (	RinboundSource!
signup_stage (	RsignupStage",
ValidateEmailRequest
email (	Remail"-
ValidateEmailResponse
valid (Rvalid"N
ValidateRegistrationCodeRequest+
registration_code (	RregistrationCode"8
 ValidateRegistrationCodeResponse
valid (Rvalid"N
JoinWaitlistRequest!
feature_name (	RfeatureName
email (	Remail".
JoinWaitlistResponse
result (	Rresult"ã
ContactFormRequest
name (	Rname
email (	Remail"
organization (	Rorganization
	job_title (	RjobTitle!
phone_number (	RphoneNumber
details (	Rdetails7
how_did_you_hear_about_us (	RhowDidYouHearAboutUs
seats (	Rseats"
ContactFormResponse".
SubscribeToBlogRequest
email (	Remail"
SubscribeToBlogResponse"4
UnsubscribeFromEmailsRequest
email (	Remail"
UnsubscribeFromEmailsResponse"i
SendReferralEmailRequest

auth_token (	R	authToken
email (	Remail
details (	Rdetails"
SendReferralEmailResponse"7
PingRequest(
work_duration_ms (RworkDurationMs"-
PingResponse

latency_ms (R	latencyMs"
WhoAmIRequest"&
HeaderValues
values (	Rvalues"À
WhoAmIResponse
ldap (	RldapH
headers (2..exa.api_server_pb.WhoAmIResponse.HeadersEntryRheaders[
HeadersEntry
key (	Rkey5
value (2.exa.api_server_pb.HeaderValuesRvalue:8" 
GetUnleashContextFieldsRequest"ï
GetUnleashContextFieldsResponse

load_score (R	loadScore
country (	Rcountry
	continent (	R	continent
	time_zone (	RtimeZone"
GetExtensionStatsRequest"ì
GetExtensionStatsResponse*
vs_code_downloads (RvsCodeDownloads-
jetbrain_downloads (RjetbrainDownloads
	vim_stars (RvimStars"é
RegisterHybridDeploymentRequest
version (	Rversion
team_id (	RteamId
base_url (	RbaseUrl

auth_token (	R	authToken""
 RegisterHybridDeploymentResponse"ç
%CreateHybridDeploymentInternalRequest
version (	Rversion
team_id (	RteamId
base_url (	RbaseUrl
secret (	Rsecret"(
&CreateHybridDeploymentInternalResponse"X
%RemoveHybridDeploymentInternalRequest
team_id (	RteamId
secret (	Rsecret"(
&RemoveHybridDeploymentInternalResponse"=
#GetHybridDeploymentsInternalRequest
secret (	Rsecret"Ö
HybridDeployment
team_id (	RteamId#
shared_secret (	RsharedSecret
base_url (	RbaseUrl
version (	Rversion"m
$GetHybridDeploymentsInternalResponseE
deployments (2#.exa.api_server_pb.HybridDeploymentRdeployments"\
"CheckHybridDeploymentStatusRequest

auth_token (	R	authToken
team_id (	RteamId"~
#CheckHybridDeploymentStatusResponseA
status (2).exa.api_server_pb.HybridDeploymentStatusRstatus
error (	Rerror"˛
LogCompletionsHybridRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadatas
completions_with_latency_info (20.exa.codeium_common_pb.CompletionWithLatencyInfoRcompletionsWithLatencyInfoN
provider_source (2%.exa.codeium_common_pb.ProviderSourceRproviderSource
	repo_name (	RrepoName#
relative_path (	RrelativePath
	model_tag (	RmodelTag"
LogCompletionsHybridResponse"π
LogFeedbackHybridRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata#
completion_id (	RcompletionId;
feedback (2.exa.api_server_pb.FeedbackTypeRfeedback"
LogFeedbackHybridResponse"ª
LogChatHybridRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata:
messages (2.exa.chat_pb.ChatMessagePromptRmessages'
conversation_id (	RconversationId!
active_repos (	RactiveRepos;
chat_message (2.exa.chat_pb.ChatMessageRchatMessage!
message_text (	RmessageText"
LogChatHybridResponse"O
GetStatusRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata"J
GetStatusResponse5
status (2.exa.codeium_common_pb.StatusRstatus"Ñ
GetCascadeModelConfigsRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
filter (H RfilteràB	
_filter"Ô
GetCascadeModelConfigsResponseZ
client_model_configs (2(.exa.codeium_common_pb.ClientModelConfigRclientModelConfigsT
client_model_sorts (2&.exa.codeium_common_pb.ClientModelSortRclientModelSortsy
default_override_model_config (21.exa.codeium_common_pb.DefaultOverrideModelConfigH RdefaultOverrideModelConfigàB 
_default_override_model_config"\
GetCommandModelConfigsRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata"|
GetCommandModelConfigsResponseZ
client_model_configs (2(.exa.codeium_common_pb.ClientModelConfigRclientModelConfigs"[
GetMcpServerTemplatesRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata"g
GetMcpServerTemplatesResponseF
	templates (2(.exa.codeium_common_pb.McpServerTemplateR	templates"¢
QueryParameterQ

parameters (21.exa.api_server_pb.QueryParameter.ParametersEntryR
parameters=
ParametersEntry
key (	Rkey
value (	Rvalue:8"å
QueryResultB
record (2*.exa.api_server_pb.QueryResult.RecordEntryRrecord9
RecordEntry
key (	Rkey
value (	Rvalue:8"ƒ
RunCodeAlignmentRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata!
file_content (	RfileContent
offset (Roffset1
rules (2.exa.codeium_common_pb.RuleRrules"`
RunCodeAlignmentResponseD

violations (2$.exa.codeium_common_pb.RuleViolationR
violations"¬
GenerateSyntheticRuleRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata!
comment_body (	RcommentBody!
file_content (	RfileContent
line_number (R
lineNumber"P
GenerateSyntheticRuleResponse/
rule (2.exa.codeium_common_pb.RuleRrule"x
FileExtensionStats
	extension (	R	extension
lines_added (R
linesAdded#
lines_deleted (RlinesDeleted"∂
	GitCommit8
	timestamp (2.google.protobuf.TimestampR	timestampW
file_extension_stats (2%.exa.api_server_pb.FileExtensionStatsRfileExtensionStats
commit (	Rcommit"¥
RecordGitTelemetryRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata=
git_commits (2.exa.api_server_pb.GitCommitR
gitCommits
	repo_name (	RrepoName"
RecordGitTelemetryResponse"ñ
RecordProfilingDataHeader;
metadata (2.exa.codeium_common_pb.MetadataRmetadata 
application (	Rapplication
resource (	Rresource"-
RecordProfilingDataData
data (Rdata"Ø
RecordProfilingDataRequestF
header (2,.exa.api_server_pb.RecordProfilingDataHeaderH Rheader@
data (2*.exa.api_server_pb.RecordProfilingDataDataH RdataB
chunk"
RecordProfilingDataResponse"ı
RecordReadUrlContentRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
url (	RurlK
web_document (2(.exa.codeium_common_pb.KnowledgeBaseItemRwebDocument

latency_ms (R	latencyMs
	is_cached (RisCached"
RecordReadUrlContentResponse"V
GetDeploymentConfigRequest8
key (2&.exa.api_server_pb.DeploymentConfigKeyRkey"¸
GetDeploymentConfigResponse
value (	Rvalue9

created_at (2.google.protobuf.TimestampR	createdAtB
last_updated_at (2.google.protobuf.TimestampRlastUpdatedAt&
last_updated_by (	RlastUpdatedBy 
description (	Rdescription"à
UpsertDeploymentConfigRequest8
key (2&.exa.api_server_pb.DeploymentConfigKeyRkey
value (	Rvalue
api_key (	RapiKey" 
UpsertDeploymentConfigResponse"/
GetUserAllowlistRequest
email (	Remail"ä
GetUserAllowlistResponse
email (	Remail

created_by (	R	createdBy9

created_at (2.google.protobuf.TimestampR	createdAt"M
InsertAllowlistRequest
email (	Remail

created_by (	R	createdBy"
InsertAllowlistResponse"G
DeleteAllowlistRequest
email (	Remail
api_key (	RapiKey"
DeleteAllowlistResponse"
GetAllowlistRequest"a
GetAllowlistResponseI
	allowlist (2+.exa.api_server_pb.GetUserAllowlistResponseR	allowlist"Õ
RecordCascadeUsageRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata#
trajectory_id (	RtrajectoryId

step_index (R	stepIndex.
prompt_credits_used (RpromptCreditsUsed*
flow_credits_used (RflowCreditsUsedS
planner_mode (20.exa.codeium_common_pb.ConversationalPlannerModeRplannerMode"q
RecordCascadeUsageResponseB
user_status (2!.exa.codeium_common_pb.UserStatusR
userStatusJR	plan_info"‘
GetWebSearchResultsRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
query (	Rquery2
model (2.exa.codeium_common_pb.ModelRmodel
limit (Rlimit
domain (	Rdomain^
third_party_config (20.exa.codeium_common_pb.ThirdPartyWebSearchConfigRthirdPartyConfig!
image_search (RimageSearch"°
GetWebSearchResultsResponseB
results (2(.exa.codeium_common_pb.KnowledgeBaseItemRresults$
web_search_url (	RwebSearchUrl
summary (	Rsummary"W
GetWebDocsOptionsRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata"[
GetWebDocsOptionsResponse>
options (2$.exa.codeium_common_pb.WebDocsOptionRoptions"@
GetWebSearchRedirectRequest!
original_url (	RoriginalUrl"A
GetWebSearchRedirectResponse!
redirect_url (	RredirectUrl"ÿ
OidcProviderConfig
	client_id (	RclientId#
client_secret (	RclientSecret

issuer_url (	R	issuerUrl!
redirect_url (	RredirectUrl9

created_at (2.google.protobuf.TimestampR	createdAtB
last_updated_at (2.google.protobuf.TimestampRlastUpdatedAt&
last_updated_by (	RlastUpdatedBy
team_id (	RteamId"í
RegisterOidcProviderRequest
api_key (	RapiKey=
config (2%.exa.api_server_pb.OidcProviderConfigRconfig
	team_name (	RteamName"
RegisterOidcProviderResponse"S
GetTeamOidcProvidersRequest
api_key (	RapiKey
	team_name (	RteamName"c
GetTeamOidcProvidersResponseC
	providers (2%.exa.api_server_pb.OidcProviderConfigR	providers"5
GetAllOidcProvidersRequest
api_key (	RapiKey"b
GetAllOidcProvidersResponseC
	providers (2%.exa.api_server_pb.OidcProviderConfigR	providers"Q
DeleteOidcProviderRequest
api_key (	RapiKey
	client_id (	RclientId"
DeleteOidcProviderResponse"=
GetOidcAuthorizationUrlRequest
	client_id (	RclientId"R
GetOidcAuthorizationUrlResponse
auth_url (	RauthUrl
state (	Rstate"`
ExchangeOidcCodeRequest
code (	Rcode
state (	Rstate
	client_id (	RclientId"}
ExchangeOidcCodeResponse!
access_token (	RaccessToken
id_token (	RidToken#
refresh_token (	RrefreshToken"[
RefreshOidcTokenRequest
	client_id (	RclientId#
refresh_token (	RrefreshToken"}
RefreshOidcTokenResponse!
access_token (	RaccessToken
id_token (	RidToken#
refresh_token (	RrefreshToken"c
$GetTeamOrganizationalControlsRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata"L
+GetTeamOrganizationalControlsForSiteRequest

auth_token (	R	authToken"v
%GetTeamOrganizationalControlsResponseM
controls (21.exa.codeium_common_pb.TeamOrganizationalControlsRcontrols"}
,GetTeamOrganizationalControlsForSiteResponseM
controls (21.exa.codeium_common_pb.TeamOrganizationalControlsRcontrols"µ
'UpsertTeamOrganizationalControlsRequestM
controls (21.exa.codeium_common_pb.TeamOrganizationalControlsRcontrols;
metadata (2.exa.codeium_common_pb.MetadataRmetadata"*
(UpsertTeamOrganizationalControlsResponse"û
.UpsertTeamOrganizationalControlsForSiteRequestM
controls (21.exa.codeium_common_pb.TeamOrganizationalControlsRcontrols

auth_token (	R	authToken"1
/UpsertTeamOrganizationalControlsForSiteResponse"f
'DeleteTeamOrganizationalControlsRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata"*
(DeleteTeamOrganizationalControlsResponse"|
 ApplyTrajectoryHeuristicsRequest

heuristics (	R
heuristics8

trajectory (2.gemini_coder.TrajectoryR
trajectory"V
!ApplyTrajectoryHeuristicsResponse1
triggered_heuristics (	RtriggeredHeuristics"‘
"CreateTrajectoryShareStreamRequestY
initial (2=.exa.api_server_pb.CreateTrajectoryShareStreamRequest.InitialH RinitialS
chunk (2;.exa.api_server_pb.CreateTrajectoryShareStreamRequest.ChunkH Rchunk”
Initial;
metadata (2.exa.codeium_common_pb.MetadataRmetadataG
share_status (2$.exa.cortex_pb.TrajectoryShareStatusRshareStatus#
trajectory_id (	RtrajectoryId

num_chunks (R	numChunks
Chunk
chunk (	RchunkB	
request"R
#CreateTrajectoryShareStreamResponse
url (	Rurl
share_id (	RshareId"u
FetchTrajectoryShareRequest
share_id (	RshareId;
metadata (2.exa.codeium_common_pb.MetadataRmetadata"±
FetchTrajectoryShareResponse8

trajectory (2.gemini_coder.TrajectoryR
trajectoryH
summary (2..exa.jetski_cortex_pb.CascadeTrajectorySummaryRsummary
	user_name (	RuserName7
user_profile_picture_url (	RuserProfilePictureUrl7
	shared_at (2.google.protobuf.TimestampRsharedAt"R
DeleteTrajectoryShareRequest
share_id (	RshareId
api_key (	RapiKey"9
DeleteTrajectoryShareResponse
success (Rsuccess"û
TrajectoryShare
share_id (	RshareId#
trajectory_id (	RtrajectoryId8

trajectory (2.gemini_coder.TrajectoryR
trajectoryH
summary (2..exa.jetski_cortex_pb.CascadeTrajectorySummaryRsummaryG
share_status (2$.exa.cortex_pb.TrajectoryShareStatusRshareStatus"<
!FetchTrajectoryShareByUserRequest
api_key (	RapiKey"∂
"FetchTrajectoryShareByUserResponse
	user_name (	RuserName7
user_profile_picture_url (	RuserProfilePictureUrl:
shares (2".exa.api_server_pb.TrajectoryShareRshares"J
#IsConversationSharingBlockedRequest

auth_token (	R	authTokenJ"@
$IsConversationSharingBlockedResponse
blocked (Rblocked"a
"GetDefaultWorkflowTemplatesRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata"`
#GetDefaultWorkflowTemplatesResponse9
	workflows (2.exa.cortex_pb.WorkflowSpecR	workflows*ù
MQueryScoreStatus$
 M_QUERY_SCORE_STATUS_UNSPECIFIED 
M_QUERY_SCORE_STATUS_OK
M_QUERY_SCORE_STATUS_ERROR%
!M_QUERY_SCORE_STATUS_EARLY_RETURN*â
EmbeddingRetryPolicy&
"EMBEDDING_RETRY_POLICY_UNSPECIFIED 
EMBEDDING_RETRY_POLICY_NONE(
$EMBEDDING_RETRY_POLICY_HIGH_PRIORITY*á
ChatMessageRequestType)
%CHAT_MESSAGE_REQUEST_TYPE_UNSPECIFIED %
!CHAT_MESSAGE_REQUEST_TYPE_GENERAL+
'CHAT_MESSAGE_REQUEST_TYPE_CONTEXT_CHECK"
CHAT_MESSAGE_REQUEST_TYPE_PLAN%
!CHAT_MESSAGE_REQUEST_TYPE_COMMAND%
!CHAT_MESSAGE_REQUEST_TYPE_CASCADE"
CHAT_MESSAGE_REQUEST_TYPE_EVAL0
,CHAT_MESSAGE_REQUEST_TYPE_ANTIGRAVITY_REVIEW$
 CHAT_MESSAGE_REQUEST_TYPE_MQUERY,
(CHAT_MESSAGE_REQUEST_TYPE_COMMIT_MESSAGE	(
$CHAT_MESSAGE_REQUEST_TYPE_CHECKPOINT
(
$CHAT_MESSAGE_REQUEST_TYPE_FAST_APPLY*å
HybridDeploymentStatus(
$HYBRID_DEPLOYMENT_STATUS_UNSPECIFIED $
 HYBRID_DEPLOYMENT_STATUS_HEALTHY"
HYBRID_DEPLOYMENT_STATUS_ERROR*a
FeedbackType
FEEDBACK_TYPE_UNSPECIFIED 
FEEDBACK_TYPE_ACCEPT
FEEDBACK_TYPE_REJECT*‚
FeedbackExtraInfo#
FEEDBACK_EXTRA_INFO_UNSPECIFIED *
&FEEDBACK_EXTRA_INFO_TYPED_AS_SUGGESTED(
$FEEDBACK_EXTRA_INFO_DOCUMENT_CHANGED&
"FEEDBACK_EXTRA_INFO_PARTIAL_ACCEPT*
&FEEDBACK_EXTRA_INFO_INTENTIONAL_REJECT*ó
DeploymentConfigKey%
!DEPLOYMENT_CONFIG_KEY_UNSPECIFIED -
)DEPLOYMENT_CONFIG_KEY_ENABLE_EMAIL_SIGNUP*
&DEPLOYMENT_CONFIG_KEY_SAML_GROUP_REGEX2Å
EvalApiServerServicei
GetChatMessage(.exa.api_server_pb.GetChatMessageRequest).exa.api_server_pb.GetChatMessageResponse" 02ôo
ApiServerService®
#GetStreamingExternalChatCompletions=.exa.api_server_pb.GetStreamingExternalChatCompletionsRequest>.exa.api_server_pb.GetStreamingExternalChatCompletionsResponse" 0d
GetEmbeddings'.exa.api_server_pb.GetEmbeddingsRequest(.exa.api_server_pb.GetEmbeddingsResponse" i
GetChatMessage(.exa.api_server_pb.GetChatMessageRequest).exa.api_server_pb.GetChatMessageResponse" 0•
"GetStreamingModelAPITextCompletion<.exa.api_server_pb.GetStreamingModelAPITextCompletionRequest=.exa.api_server_pb.GetStreamingModelAPITextCompletionResponse" 0î
GetTeamOrganizationalControls7.exa.api_server_pb.GetTeamOrganizationalControlsRequest8.exa.api_server_pb.GetTeamOrganizationalControlsResponse" ©
$GetTeamOrganizationalControlsForSite>.exa.api_server_pb.GetTeamOrganizationalControlsForSiteRequest?.exa.api_server_pb.GetTeamOrganizationalControlsForSiteResponse" ù
 UpsertTeamOrganizationalControls:.exa.api_server_pb.UpsertTeamOrganizationalControlsRequest;.exa.api_server_pb.UpsertTeamOrganizationalControlsResponse" ≤
'UpsertTeamOrganizationalControlsForSiteA.exa.api_server_pb.UpsertTeamOrganizationalControlsForSiteRequestB.exa.api_server_pb.UpsertTeamOrganizationalControlsForSiteResponse" ù
 DeleteTeamOrganizationalControls:.exa.api_server_pb.DeleteTeamOrganizationalControlsRequest;.exa.api_server_pb.DeleteTeamOrganizationalControlsResponse" X
	GetMQuery#.exa.api_server_pb.GetMQueryRequest$.exa.api_server_pb.GetMQueryResponse" j
ProvideFeedback).exa.api_server_pb.ProvideFeedbackRequest*.exa.api_server_pb.ProvideFeedbackResponse" p
UploadErrorTraces+.exa.api_server_pb.UploadErrorTracesRequest,.exa.api_server_pb.UploadErrorTracesResponse" v
RecordOpportunities-.exa.api_server_pb.RecordOpportunitiesRequest..exa.api_server_pb.RecordOpportunitiesResponse" Ö
RecordCodeTrackerUpdates2.exa.api_server_pb.RecordCodeTrackerUpdatesRequest3.exa.api_server_pb.RecordCodeTrackerUpdatesResponse" Ç
RecordCompletionExample1.exa.api_server_pb.RecordCompletionExampleRequest2.exa.api_server_pb.RecordCompletionExampleResponse" |
GetCompletionExamples/.exa.api_server_pb.GetCompletionExamplesRequest0.exa.api_server_pb.GetCompletionExamplesResponse" n
RecordCompletions*.exa.analytics_pb.RecordCompletionsRequest+.exa.analytics_pb.RecordCompletionsResponse" y
RecordAsyncTelemetry..exa.api_server_pb.RecordAsyncTelemetryRequest/.exa.api_server_pb.RecordAsyncTelemetryResponse" [

RecordChat$.exa.api_server_pb.RecordChatRequest%.exa.api_server_pb.RecordChatResponse" s
RecordChatFeedback,.exa.api_server_pb.RecordChatFeedbackRequest-.exa.api_server_pb.RecordChatFeedbackResponse" 
RecordChatPanelSession0.exa.api_server_pb.RecordChatPanelSessionRequest1.exa.api_server_pb.RecordChatPanelSessionResponse" y
RecordContextRefresh..exa.api_server_pb.RecordContextRefreshRequest/.exa.api_server_pb.RecordContextRefreshResponse" z
RecordContextToPrompt..exa.analytics_pb.RecordContextToPromptRequest/.exa.analytics_pb.RecordContextToPromptResponse" ^
RecordEvent%.exa.api_server_pb.RecordEventRequest&.exa.api_server_pb.RecordEventResponse" a
RecordSearch&.exa.api_server_pb.RecordSearchRequest'.exa.api_server_pb.RecordSearchResponse" v
RecordSearchResults-.exa.api_server_pb.RecordSearchResultsRequest..exa.api_server_pb.RecordSearchResultsResponse" v
RecordSearchDocOpen-.exa.api_server_pb.RecordSearchDocOpenRequest..exa.api_server_pb.RecordSearchDocOpenResponse" Ç
RecordSearchResultsView1.exa.api_server_pb.RecordSearchResultsViewRequest2.exa.api_server_pb.RecordSearchResultsViewResponse" g
RecordDebounce(.exa.api_server_pb.RecordDebounceRequest).exa.api_server_pb.RecordDebounceResponse" v
RecordPinnedContext-.exa.api_server_pb.RecordPinnedContextRequest..exa.api_server_pb.RecordPinnedContextResponse" q
RecordCommandUsage+.exa.analytics_pb.RecordCommandUsageRequest,.exa.analytics_pb.RecordCommandUsageResponse" 
RecordChatModelNodeRun0.exa.api_server_pb.RecordChatModelNodeRunRequest1.exa.api_server_pb.RecordChatModelNodeRunResponse" a
RecordMQuery&.exa.api_server_pb.RecordMQueryRequest'.exa.api_server_pb.RecordMQueryResponse" m
RecordCortexStep*.exa.api_server_pb.RecordCortexStepRequest+.exa.api_server_pb.RecordCortexStepResponse" î
RecordCommitMessageGeneration7.exa.api_server_pb.RecordCommitMessageGenerationRequest8.exa.api_server_pb.RecordCommitMessageGenerationResponse" Ç
RecordCommitMessageSave1.exa.api_server_pb.RecordCommitMessageSaveRequest2.exa.api_server_pb.RecordCommitMessageSaveResponse" s
RecordGitTelemetry,.exa.api_server_pb.RecordGitTelemetryRequest-.exa.api_server_pb.RecordGitTelemetryResponse" x
RecordProfilingData-.exa.api_server_pb.RecordProfilingDataRequest..exa.api_server_pb.RecordProfilingDataResponse" (y
RecordReadUrlContent..exa.api_server_pb.RecordReadUrlContentRequest/.exa.api_server_pb.RecordReadUrlContentResponse" v
RecordNewCortexPlan-.exa.api_server_pb.RecordNewCortexPlanRequest..exa.api_server_pb.RecordNewCortexPlanResponse" 
RecordCortexCodingPlan0.exa.api_server_pb.RecordCortexCodingPlanRequest1.exa.api_server_pb.RecordCortexCodingPlanResponse" 
RecordCortexCodingStep0.exa.api_server_pb.RecordCortexCodingStepRequest1.exa.api_server_pb.RecordCortexCodingStepResponse" ó
RecordCortexCodingStepFeedback8.exa.api_server_pb.RecordCortexCodingStepFeedbackRequest9.exa.api_server_pb.RecordCortexCodingStepFeedbackResponse" y
RecordCortexFeedback..exa.api_server_pb.RecordCortexFeedbackRequest/.exa.api_server_pb.RecordCortexFeedbackResponse" p
RecordCortexError+.exa.api_server_pb.RecordCortexErrorRequest,.exa.api_server_pb.RecordCortexErrorResponse" }
RecordCortexTrajectory/.exa.analytics_pb.RecordCortexTrajectoryRequest0.exa.analytics_pb.RecordCortexTrajectoryResponse" â
RecordCortexTrajectoryStep3.exa.analytics_pb.RecordCortexTrajectoryStepRequest4.exa.analytics_pb.RecordCortexTrajectoryStepResponse" î
RecordCortexGeneratorMetadata7.exa.api_server_pb.RecordCortexGeneratorMetadataRequest8.exa.api_server_pb.RecordCortexGeneratorMetadataResponse" î
RecordCortexExecutionMetadata7.exa.api_server_pb.RecordCortexExecutionMetadataRequest8.exa.api_server_pb.RecordCortexExecutionMetadataResponse" î
RecordStateInitializationData7.exa.api_server_pb.RecordStateInitializationDataRequest8.exa.api_server_pb.RecordStateInitializationDataResponse" é
GetDefaultWorkflowTemplates5.exa.api_server_pb.GetDefaultWorkflowTemplatesRequest6.exa.api_server_pb.GetDefaultWorkflowTemplatesResponse" q
BatchRecordPrompts+.exa.analytics_pb.BatchRecordPromptsRequest,.exa.analytics_pb.BatchRecordPromptsResponse" }
BatchRecordCompletions/.exa.analytics_pb.BatchRecordCompletionsRequest0.exa.analytics_pb.BatchRecordCompletionsResponse" ó
BatchRecordUserLastUpdateTimes8.exa.api_server_pb.BatchRecordUserLastUpdateTimesRequest9.exa.api_server_pb.BatchRecordUserLastUpdateTimesResponse" î
BatchRecordChatRequestRecords7.exa.api_server_pb.BatchRecordChatRequestRecordsRequest8.exa.api_server_pb.BatchRecordChatRequestRecordsResponse" I
Ping.exa.api_server_pb.PingRequest.exa.api_server_pb.PingResponse" O
WhoAmI .exa.api_server_pb.WhoAmIRequest!.exa.api_server_pb.WhoAmIResponse" X
	Subscribe#.exa.api_server_pb.SubscribeRequest$.exa.api_server_pb.SubscribeResponse" d
ValidateEmail'.exa.api_server_pb.ValidateEmailRequest(.exa.api_server_pb.ValidateEmailResponse" Ö
ValidateRegistrationCode2.exa.api_server_pb.ValidateRegistrationCodeRequest3.exa.api_server_pb.ValidateRegistrationCodeResponse" a
JoinWaitlist&.exa.api_server_pb.JoinWaitlistRequest'.exa.api_server_pb.JoinWaitlistResponse" ^
ContactForm%.exa.api_server_pb.ContactFormRequest&.exa.api_server_pb.ContactFormResponse" p
GetExtensionStats+.exa.api_server_pb.GetExtensionStatsRequest,.exa.api_server_pb.GetExtensionStatsResponse" j
SubscribeToBlog).exa.api_server_pb.SubscribeToBlogRequest*.exa.api_server_pb.SubscribeToBlogResponse" |
UnsubscribeFromEmails/.exa.api_server_pb.UnsubscribeFromEmailsRequest0.exa.api_server_pb.UnsubscribeFromEmailsResponse" p
SendReferralEmail+.exa.api_server_pb.SendReferralEmailRequest,.exa.api_server_pb.SendReferralEmailResponse" m
RunCodeAlignment*.exa.api_server_pb.RunCodeAlignmentRequest+.exa.api_server_pb.RunCodeAlignmentResponse" |
GenerateSyntheticRule/.exa.api_server_pb.GenerateSyntheticRuleRequest0.exa.api_server_pb.GenerateSyntheticRuleResponse" m
GetUserAllowlist*.exa.api_server_pb.GetUserAllowlistRequest+.exa.api_server_pb.GetUserAllowlistResponse" j
InsertAllowlist).exa.api_server_pb.InsertAllowlistRequest*.exa.api_server_pb.InsertAllowlistResponse" j
DeleteAllowlist).exa.api_server_pb.DeleteAllowlistRequest*.exa.api_server_pb.DeleteAllowlistResponse" a
GetAllowlist&.exa.api_server_pb.GetAllowlistRequest'.exa.api_server_pb.GetAllowlistResponse" Ö
RegisterHybridDeployment2.exa.api_server_pb.RegisterHybridDeploymentRequest3.exa.api_server_pb.RegisterHybridDeploymentResponse" ó
CreateHybridDeploymentInternal8.exa.api_server_pb.CreateHybridDeploymentInternalRequest9.exa.api_server_pb.CreateHybridDeploymentInternalResponse" ó
RemoveHybridDeploymentInternal8.exa.api_server_pb.RemoveHybridDeploymentInternalRequest9.exa.api_server_pb.RemoveHybridDeploymentInternalResponse" ë
GetHybridDeploymentsInternal6.exa.api_server_pb.GetHybridDeploymentsInternalRequest7.exa.api_server_pb.GetHybridDeploymentsInternalResponse" é
CheckHybridDeploymentStatus5.exa.api_server_pb.CheckHybridDeploymentStatusRequest6.exa.api_server_pb.CheckHybridDeploymentStatusResponse" y
LogCompletionsHybrid..exa.api_server_pb.LogCompletionsHybridRequest/.exa.api_server_pb.LogCompletionsHybridResponse" p
LogFeedbackHybrid+.exa.api_server_pb.LogFeedbackHybridRequest,.exa.api_server_pb.LogFeedbackHybridResponse" d
LogChatHybrid'.exa.api_server_pb.LogChatHybridRequest(.exa.api_server_pb.LogChatHybridResponse" X
	GetStatus#.exa.api_server_pb.GetStatusRequest$.exa.api_server_pb.GetStatusResponse" 
GetCascadeModelConfigs0.exa.api_server_pb.GetCascadeModelConfigsRequest1.exa.api_server_pb.GetCascadeModelConfigsResponse" Ç
GetCommandModelConfigs0.exa.api_server_pb.GetCommandModelConfigsRequest1.exa.api_server_pb.GetCommandModelConfigsResponse"à|
GetMcpServerTemplates/.exa.api_server_pb.GetMcpServerTemplatesRequest0.exa.api_server_pb.GetMcpServerTemplatesResponse" Ç
GetUnleashContextFields1.exa.api_server_pb.GetUnleashContextFieldsRequest2.exa.api_server_pb.GetUnleashContextFieldsResponse" ù
 RecordTrajectorySegmentAnalytics:.exa.api_server_pb.RecordTrajectorySegmentAnalyticsRequest;.exa.api_server_pb.RecordTrajectorySegmentAnalyticsResponse" î
RecordFullTrajectoryAnalytics7.exa.api_server_pb.RecordFullTrajectoryAnalyticsRequest8.exa.api_server_pb.RecordFullTrajectoryAnalyticsResponse" î
RecordTrajectorySegmentEvents7.exa.api_server_pb.RecordTrajectorySegmentEventsRequest8.exa.api_server_pb.RecordTrajectorySegmentEventsResponse" 
SupportsRemoteIndexing0.exa.api_server_pb.SupportsRemoteIndexingRequest1.exa.api_server_pb.SupportsRemoteIndexingResponse" m
GetModelStatuses*.exa.api_server_pb.GetModelStatusesRequest+.exa.api_server_pb.GetModelStatusesResponse" d
GetModelInfos'.exa.api_server_pb.GetModelInfosRequest(.exa.api_server_pb.GetModelInfosResponse" v
GetDeploymentConfig-.exa.api_server_pb.GetDeploymentConfigRequest..exa.api_server_pb.GetDeploymentConfigResponse" 
UpsertDeploymentConfig0.exa.api_server_pb.UpsertDeploymentConfigRequest1.exa.api_server_pb.UpsertDeploymentConfigResponse" s
RecordCascadeUsage,.exa.api_server_pb.RecordCascadeUsageRequest-.exa.api_server_pb.RecordCascadeUsageResponse" à
ApplyTrajectoryHeuristics3.exa.api_server_pb.ApplyTrajectoryHeuristicsRequest4.exa.api_server_pb.ApplyTrajectoryHeuristicsResponse" v
GetWebSearchResults-.exa.api_server_pb.GetWebSearchResultsRequest..exa.api_server_pb.GetWebSearchResultsResponse" p
GetWebDocsOptions+.exa.api_server_pb.GetWebDocsOptionsRequest,.exa.api_server_pb.GetWebDocsOptionsResponse" y
GetWebSearchRedirect..exa.api_server_pb.GetWebSearchRedirectRequest/.exa.api_server_pb.GetWebSearchRedirectResponse" m
GetTranscription*.exa.api_server_pb.GetTranscriptionRequest+.exa.api_server_pb.GetTranscriptionResponse" s
GetImageGeneration,.exa.api_server_pb.GetImageGenerationRequest-.exa.api_server_pb.GetImageGenerationResponse" y
RegisterOidcProvider..exa.api_server_pb.RegisterOidcProviderRequest/.exa.api_server_pb.RegisterOidcProviderResponse" y
GetTeamOidcProviders..exa.api_server_pb.GetTeamOidcProvidersRequest/.exa.api_server_pb.GetTeamOidcProvidersResponse" v
GetAllOidcProviders-.exa.api_server_pb.GetAllOidcProvidersRequest..exa.api_server_pb.GetAllOidcProvidersResponse" s
DeleteOidcProvider,.exa.api_server_pb.DeleteOidcProviderRequest-.exa.api_server_pb.DeleteOidcProviderResponse" Ç
GetOidcAuthorizationUrl1.exa.api_server_pb.GetOidcAuthorizationUrlRequest2.exa.api_server_pb.GetOidcAuthorizationUrlResponse" m
ExchangeOidcCode*.exa.api_server_pb.ExchangeOidcCodeRequest+.exa.api_server_pb.ExchangeOidcCodeResponse" m
RefreshOidcToken*.exa.api_server_pb.RefreshOidcTokenRequest+.exa.api_server_pb.RefreshOidcTokenResponse" ê
CreateTrajectoryShareStream5.exa.api_server_pb.CreateTrajectoryShareStreamRequest6.exa.api_server_pb.CreateTrajectoryShareStreamResponse" (y
FetchTrajectoryShare..exa.api_server_pb.FetchTrajectoryShareRequest/.exa.api_server_pb.FetchTrajectoryShareResponse" |
DeleteTrajectoryShare/.exa.api_server_pb.DeleteTrajectoryShareRequest0.exa.api_server_pb.DeleteTrajectoryShareResponse" ã
FetchTrajectoryShareByUser4.exa.api_server_pb.FetchTrajectoryShareByUserRequest5.exa.api_server_pb.FetchTrajectoryShareByUserResponse" j
GetCascadeNuxes).exa.api_server_pb.GetCascadeNuxesRequest*.exa.api_server_pb.GetCascadeNuxesResponse" ë
IsConversationSharingBlocked6.exa.api_server_pb.IsConversationSharingBlockedRequest7.exa.api_server_pb.IsConversationSharingBlockedResponse" f
StreamingTest'.exa.api_server_pb.StreamingTestRequest(.exa.api_server_pb.StreamingTestResponse" 0BÚOPEN_TO_OPAQUE_HYBRIDbproto3